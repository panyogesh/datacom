#!/usr/bin/env bash
set -euo pipefail

# kvm_lab_2vms_3nics.sh
#
# Create/cleanup 2 VMs with 3 NICs each:
#   NIC0: NAT network (DHCP internet) on libvirt 'default' (or 'nat0' if default missing)
#   NIC1: netA (isolated) with static IP
#   NIC2: netB (isolated) with static IP
#
# Networks are derived from CIDRs:
#   netA name  = net<3rd_octet>   bridge = virbr<3rd_octet>
#   netB name  = net<3rd_octet>   bridge = virbr<3rd_octet>
#
# Only TWO IP inputs are required: ip1 and ip2 (for VM1 and VM2 host octet or full IP on netA).
# netB will use the SAME host octets as netA.
#
# Examples:
#   sudo ./kvm_lab_2vms_3nics.sh create \
#     --pubkey "ssh-ed25519 AAAA... user@host" \
#     --prefix mi-test-vm- \
#     --netA 192.168.58.0/24 \
#     --netB 192.168.59.0/24 \
#     --ip1 197 --ip2 199
#
#   sudo ./kvm_lab_2vms_3nics.sh cleanup \
#     --prefix mi-test-vm- \
#     --netA 192.168.58.0/24 \
#     --netB 192.168.59.0/24

usage() {
  cat <<'USAGE'
Usage:
  kvm_lab_2vms_3nics.sh create  --pubkey "<SSH_PUBLIC_KEY>" --prefix "<PREFIX>" --netA "<CIDR>" --netB "<CIDR>" --ip1 "<IP|HOST>" --ip2 "<IP|HOST>" [options]
  kvm_lab_2vms_3nics.sh cleanup --prefix "<PREFIX>" --netA "<CIDR>" --netB "<CIDR>" [options]

Required (create):
  --pubkey   "<SSH_PUBLIC_KEY_LINE>"
  --prefix   "<VM_PREFIX>"        -> creates ${prefix}1 and ${prefix}2
  --netA     "192.168.X.0/24"
  --netB     "192.168.Y.0/24"
  --ip1      "197" or "192.168.X.197"  (VM1 IP on netA)
  --ip2      "199" or "192.168.X.199"  (VM2 IP on netA)

Required (cleanup):
  --prefix, --netA, --netB

Options:
  --ram-mb   5120
  --vcpus    4
  --disk-gb  40
  --base-dir /var/lib/libvirt/images
  --os-variant ubuntu22.04
  --nat-net  default          # if missing, script creates/uses nat0 (does NOT delete NAT on cleanup)
  --force                   # delete/recreate VMs if they already exist (create mode)
USAGE
}

ACTION="${1:-}"
shift || true

PUBKEY=""
PREFIX=""
NETA_CIDR=""
NETB_CIDR=""
IP1=""
IP2=""
RAM_MB=5120
VCPUS=4
DISK_GB=40
BASE_DIR="/var/lib/libvirt/images"
OS_VARIANT="ubuntu22.04"
NAT_NET="default"
FORCE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --pubkey) PUBKEY="${2:-}"; shift 2;;
    --prefix) PREFIX="${2:-}"; shift 2;;
    --netA) NETA_CIDR="${2:-}"; shift 2;;
    --netB) NETB_CIDR="${2:-}"; shift 2;;
    --ip1) IP1="${2:-}"; shift 2;;
    --ip2) IP2="${2:-}"; shift 2;;
    --ram-mb) RAM_MB="${2:-}"; shift 2;;
    --vcpus) VCPUS="${2:-}"; shift 2;;
    --disk-gb) DISK_GB="${2:-}"; shift 2;;
    --base-dir) BASE_DIR="${2:-}"; shift 2;;
    --os-variant) OS_VARIANT="${2:-}"; shift 2;;
    --nat-net) NAT_NET="${2:-}"; shift 2;;
    --force) FORCE=1; shift 1;;
    -h|--help) usage; exit 0;;
    *) echo "Unknown arg: $1"; usage; exit 1;;
  esac
done

need_cmd() { command -v "$1" >/dev/null 2>&1 || { echo "ERROR: missing command: $1"; exit 1; }; }

need_cmd virsh
need_cmd qemu-img

if [[ "$ACTION" == "create" ]]; then
  need_cmd virt-install
  need_cmd cloud-localds
  need_cmd wget || true
  need_cmd curl || true
elif [[ "$ACTION" == "cleanup" ]]; then
  : # ok
else
  echo "ERROR: first arg must be 'create' or 'cleanup'"
  usage
  exit 1
fi

# CIDR helpers (expects /24)
cidr_base() { echo "$1" | cut -d/ -f1; }
cidr_prefix() { echo "$1" | cut -d/ -f2; }
ip_3oct() { echo "$1" | awk -F. '{print $1"."$2"."$3}'; }
oct3() { echo "$1" | awk -F. '{print $3}'; }
host_part() {
  local in="$1"
  if [[ "$in" =~ ^[0-9]+$ ]]; then echo "$in"; else echo "$in" | awk -F. '{print $4}'; fi
}
tohex2() { printf "%02x" "$1"; }

net_exists() { virsh net-info "$1" >/dev/null 2>&1; }
net_active() { virsh net-info "$1" 2>/dev/null | awk -F: '/Active/ {gsub(/ /,"",$2); print $2}'; }

define_net_if_missing() {
  local name="$1" xml="$2"
  if net_exists "$name"; then
    echo "[=] Network '$name' already defined."
  else
    echo "[+] Defining network '$name'..."
    virsh net-define "$xml"
  fi
  if [[ "$(net_active "$name")" != "yes" ]]; then
    echo "[+] Starting network '$name'..."
    virsh net-start "$name"
  fi
  virsh net-autostart "$name" >/dev/null || true
}

dom_exists() { virsh dominfo "$1" >/dev/null 2>&1; }

BASE_IMG_DIR="${BASE_DIR}/base"
VMS_DIR="${BASE_DIR}/vms"
SEED_DIR="${BASE_DIR}/seed"
mkdir -p "$BASE_IMG_DIR" "$VMS_DIR" "$SEED_DIR"

cleanup_vm_files() {
  local vm="$1"
  rm -f  "${VMS_DIR}/${vm}.qcow2" || true
  rm -f  "${SEED_DIR}/${vm}-seed.iso" || true
  rm -rf "${SEED_DIR}/${vm}" || true
}

# Validate required args
if [[ -z "$PREFIX" || -z "$NETA_CIDR" || -z "$NETB_CIDR" ]]; then
  echo "ERROR: --prefix, --netA, --netB are required."
  usage
  exit 1
fi
if [[ "$ACTION" == "create" ]]; then
  if [[ -z "$PUBKEY" || -z "$IP1" || -z "$IP2" ]]; then
    echo "ERROR: create requires --pubkey, --ip1, --ip2."
    usage
    exit 1
  fi
fi

# /24 check
NETA_PFX="$(cidr_prefix "$NETA_CIDR")"
NETB_PFX="$(cidr_prefix "$NETB_CIDR")"
if [[ "$NETA_PFX" != "24" || "$NETB_PFX" != "24" ]]; then
  echo "ERROR: This script currently supports only /24 for netA/netB."
  echo "       netA=$NETA_CIDR netB=$NETB_CIDR"
  exit 1
fi

NETA_BASE="$(cidr_base "$NETA_CIDR")"
NETB_BASE="$(cidr_base "$NETB_CIDR")"
NETA_3="$(ip_3oct "$NETA_BASE")"
NETB_3="$(ip_3oct "$NETB_BASE")"

NETA_ID="$(oct3 "$NETA_BASE")"   # e.g. 58
NETB_ID="$(oct3 "$NETB_BASE")"   # e.g. 59

NETA_NAME="net${NETA_ID}"
NETB_NAME="net${NETB_ID}"
BR_A="virbr${NETA_ID}"
BR_B="virbr${NETB_ID}"

VM1="${PREFIX}1"
VM2="${PREFIX}2"

if [[ "$ACTION" == "cleanup" ]]; then
  echo "[+] Cleaning VMs: $VM1, $VM2"
  virsh destroy "$VM1" 2>/dev/null || true
  virsh destroy "$VM2" 2>/dev/null || true
  virsh undefine "$VM1" --nvram 2>/dev/null || virsh undefine "$VM1" 2>/dev/null || true
  virsh undefine "$VM2" --nvram 2>/dev/null || virsh undefine "$VM2" 2>/dev/null || true

  echo "[+] Removing VM disks/seeds under: $BASE_DIR"
  cleanup_vm_files "$VM1"
  cleanup_vm_files "$VM2"

  echo "[+] Cleaning networks: $NETA_NAME ($BR_A), $NETB_NAME ($BR_B)"
  virsh net-destroy "$NETA_NAME" 2>/dev/null || true
  virsh net-undefine "$NETA_NAME" 2>/dev/null || true
  virsh net-destroy "$NETB_NAME" 2>/dev/null || true
  virsh net-undefine "$NETB_NAME" 2>/dev/null || true

  echo
  echo "[+] Cleanup done."
  echo "Remaining VMs:"; virsh list --all
  echo "Remaining networks:"; virsh net-list --all
  exit 0
fi

# ----- CREATE MODE -----

H1="$(host_part "$IP1")"
H2="$(host_part "$IP2")"
for h in "$H1" "$H2"; do
  if ! [[ "$h" =~ ^[0-9]+$ ]] || ((h < 2 || h > 254)); then
    echo "ERROR: invalid host octet '$h' (must be 2..254)."
    exit 1
  fi
done

VM1_A_IP="${NETA_3}.${H1}"
VM2_A_IP="${NETA_3}.${H2}"
VM1_B_IP="${NETB_3}.${H1}"
VM2_B_IP="${NETB_3}.${H2}"

# Deterministic, prefix-unique MACs (avoid collisions across different labs)
H1H="$(tohex2 "$H1")"
H2H="$(tohex2 "$H2")"

# two bytes derived from PREFIX (stable per prefix)
SALT_HEX="$(echo -n "$PREFIX" | md5sum | awk '{print $1}')"
P1="${SALT_HEX:0:2}"
P2="${SALT_HEX:2:2}"

# network IDs as hex bytes
AID_HEX="$(printf "%02x" "$NETA_ID")"
BID_HEX="$(printf "%02x" "$NETB_ID")"

# NAT MACs: vary by prefix + VM index
VM1_NAT_MAC="52:54:00:${P1}:${P2}:01"
VM2_NAT_MAC="52:54:00:${P1}:${P2}:02"

# netA/netB MACs: vary by prefix + net-id + host octet
VM1_A_MAC="52:54:00:${P1}:${AID_HEX}:${H1H}"
VM2_A_MAC="52:54:00:${P1}:${AID_HEX}:${H2H}"
VM1_B_MAC="52:54:00:${P1}:${BID_HEX}:${H1H}"
VM2_B_MAC="52:54:00:${P1}:${BID_HEX}:${H2H}"

echo "[+] Plan:"
echo "    $VM1: nat(DHCP) + ${NETA_NAME}=${VM1_A_IP} + ${NETB_NAME}=${VM1_B_IP}"
echo "    $VM2: nat(DHCP) + ${NETA_NAME}=${VM2_A_IP} + ${NETB_NAME}=${VM2_B_IP}"
echo

# Ensure NAT network exists/active. If missing, create nat0.
if net_exists "$NAT_NET"; then
  if [[ "$(net_active "$NAT_NET")" != "yes" ]]; then
    echo "[+] Starting NAT network '$NAT_NET'..."
    virsh net-start "$NAT_NET"
  fi
  virsh net-autostart "$NAT_NET" >/dev/null || true
  echo "[+] Using NAT network: $NAT_NET"
else
  echo "[!] NAT network '$NAT_NET' not found. Creating 'nat0'..."
  NAT_NET="nat0"
  cat >/var/tmp/nat0.xml <<'XML'
<network>
  <name>nat0</name>
  <forward mode='nat'/>
  <bridge name='virbr122' stp='on' delay='0'/>
  <ip address='192.168.122.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.122.100' end='192.168.122.254'/>
    </dhcp>
  </ip>
</network>
XML
  define_net_if_missing nat0 /var/tmp/nat0.xml
  echo "[+] Using NAT network: nat0"
fi

# Create/Start netA and netB with bridge names matching (virbr58/virbr59, etc.)
cat >/var/tmp/"${NETA_NAME}".xml <<XML
<network>
  <name>${NETA_NAME}</name>
  <forward mode='none'/>
  <bridge name='${BR_A}' stp='on' delay='0'/>
  <ip address='${NETA_3}.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='${NETA_3}.10' end='${NETA_3}.190'/>
    </dhcp>
  </ip>
</network>
XML

cat >/var/tmp/"${NETB_NAME}".xml <<XML
<network>
  <name>${NETB_NAME}</name>
  <forward mode='none'/>
  <bridge name='${BR_B}' stp='on' delay='0'/>
  <ip address='${NETB_3}.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='${NETB_3}.10' end='${NETB_3}.190'/>
    </dhcp>
  </ip>
</network>
XML

define_net_if_missing "$NETA_NAME" /var/tmp/"${NETA_NAME}".xml
define_net_if_missing "$NETB_NAME" /var/tmp/"${NETB_NAME}".xml

# Base cloud image
BASE_IMG="${BASE_IMG_DIR}/jammy.qcow2"
if [[ ! -f "$BASE_IMG" ]]; then
  echo "[+] Downloading Ubuntu Jammy cloud image..."
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$BASE_IMG" https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img
  else
    wget -O "$BASE_IMG" https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img
  fi
else
  echo "[=] Base image exists: $BASE_IMG"
fi

mk_overlay() {
  local vm="$1"
  local out="${VMS_DIR}/${vm}.qcow2"
  if [[ -f "$out" ]]; then
    echo "[=] Disk exists: $out"
    return
  fi
  echo "[+] Creating disk for $vm (${DISK_GB}G)..."
  qemu-img create -f qcow2 -F qcow2 -b "$BASE_IMG" "$out" "${DISK_GB}G"
}

mk_seed() {
  local vm="$1"
  local nat_mac="$2" a_mac="$3" b_mac="$4"
  local ip_a="$5" ip_b="$6"

  local vm_seed_dir="${SEED_DIR}/${vm}"
  local seed_iso="${SEED_DIR}/${vm}-seed.iso"
  mkdir -p "$vm_seed_dir"

  cat >"${vm_seed_dir}/user-data" <<YAML
#cloud-config
hostname: ${vm}
manage_etc_hosts: true
users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ${PUBKEY}
ssh_pwauth: false
package_update: true
packages:
  - net-tools
  - iproute2
YAML

  cat >"${vm_seed_dir}/meta-data" <<YAML
instance-id: ${vm}
local-hostname: ${vm}
YAML

  # NAT NIC => DHCP + default route
  # netA/netB => static, no default route
  cat >"${vm_seed_dir}/network-config" <<YAML
version: 2
ethernets:
  nat0:
    match:
      macaddress: "${nat_mac}"
    set-name: "nat0"
    dhcp4: true
    dhcp4-overrides:
      route-metric: 50

  netA:
    match:
      macaddress: "${a_mac}"
    set-name: "netA"
    addresses: ["${ip_a}/24"]
    dhcp4: false

  netB:
    match:
      macaddress: "${b_mac}"
    set-name: "netB"
    addresses: ["${ip_b}/24"]
    dhcp4: false
YAML

  echo "[+] Creating seed ISO for $vm: $seed_iso"
  cloud-localds -v --network-config="${vm_seed_dir}/network-config" \
    "$seed_iso" "${vm_seed_dir}/user-data" "${vm_seed_dir}/meta-data"
}

mk_vm() {
  local vm="$1"
  local nat_mac="$2" a_mac="$3" b_mac="$4"

  local disk="${VMS_DIR}/${vm}.qcow2"
  local seed="${SEED_DIR}/${vm}-seed.iso"

  if dom_exists "$vm"; then
    if [[ "$FORCE" -eq 1 ]]; then
      echo "[!] VM exists, forcing recreate: $vm"
      virsh destroy "$vm" 2>/dev/null || true
      virsh undefine "$vm" --nvram 2>/dev/null || virsh undefine "$vm" 2>/dev/null || true
      cleanup_vm_files "$vm"
      mk_overlay "$vm"
      mk_seed "$vm" "$nat_mac" "$a_mac" "$b_mac" \
        "$( [[ "$vm" == "$VM1" ]] && echo "$VM1_A_IP" || echo "$VM2_A_IP" )" \
        "$( [[ "$vm" == "$VM1" ]] && echo "$VM1_B_IP" || echo "$VM2_B_IP" )"
    else
      echo "[=] Domain exists: $vm (skipping creation). Use --force to recreate."
      return
    fi
  fi

  echo "[+] Creating VM: $vm"
  virt-install \
    --name "$vm" \
    --memory "$RAM_MB" --vcpus "$VCPUS" \
    --disk "path=${disk},format=qcow2,bus=virtio" \
    --disk "path=${seed},device=cdrom" \
    --os-variant "$OS_VARIANT" \
    --import \
    --network "network=${NAT_NET},model=virtio,mac=${nat_mac}" \
    --network "network=${NETA_NAME},model=virtio,mac=${a_mac}" \
    --network "network=${NETB_NAME},model=virtio,mac=${b_mac}" \
    --graphics none \
    --console pty,target_type=serial \
    --noautoconsole
}

# Build artifacts for VM1 & VM2
mk_overlay "$VM1"
mk_overlay "$VM2"
mk_seed "$VM1" "$VM1_NAT_MAC" "$VM1_A_MAC" "$VM1_B_MAC" "$VM1_A_IP" "$VM1_B_IP"
mk_seed "$VM2" "$VM2_NAT_MAC" "$VM2_A_MAC" "$VM2_B_MAC" "$VM2_A_IP" "$VM2_B_IP"
mk_vm "$VM1" "$VM1_NAT_MAC" "$VM1_A_MAC" "$VM1_B_MAC"
mk_vm "$VM2" "$VM2_NAT_MAC" "$VM2_A_MAC" "$VM2_B_MAC"

echo
echo "[+] Done."
echo "Networks:"
echo "  $NETA_NAME bridge=$BR_A gw=${NETA_3}.1"
echo "  $NETB_NAME bridge=$BR_B gw=${NETB_3}.1"
echo "VMs:"
echo "  $VM1: ${NETA_NAME}=$VM1_A_IP, ${NETB_NAME}=$VM1_B_IP (NAT via '$NAT_NET')"
echo "  $VM2: ${NETA_NAME}=$VM2_A_IP, ${NETB_NAME}=$VM2_B_IP (NAT via '$NAT_NET')"
echo
echo "Check bridges:"
echo "  ip link show $BR_A || true"
echo "  ip link show $BR_B || true"
echo
echo "SSH (use netA IPs):"
echo "  ssh ubuntu@${VM1_A_IP}"
echo "  ssh ubuntu@${VM2_A_IP}"
