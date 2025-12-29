sudo ./yogesh-kvm-2vms-3nics.sh create \
  --pubkey "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKAi2o0MT0mqpG7COvP9Uk5fJ8BaO7SZ8OCZVE4Rn7JE wavelabs@wavelabs" \
  --prefix yo-test-vm- \
  --netA 192.168.60.0/24 \
  --netB 192.168.61.0/24 \
  --ip1 197 --ip2 199 \
  --ram-mb 10240 --vcpus 8 --disk-gb 50


sudo ./yogesh-kvm-2vms-3nics.sh cleanup \
  --prefix yo-test-vm- \
  --netA 192.168.60.0/24 \
  --netB 192.168.61.0/24

## Inside the VM
sudo mkdir -p /mnt/hostshare
sudo mount -t virtiofs hostshare /mnt/hostshare
