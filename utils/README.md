### Quick Commands

- sudo virsh list --all
- Destroy the VMs
```
sudo ./kvm_lab_2vms_3nics.sh cleanup \
  --prefix mi-test-vm- \
  --netA 192.168.58.0/24 \
  --netB 192.168.59.0/24
```
- Create VMs
```
sudo ./kvm_lab_2vms_3nics.sh create \
  --pubkey "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKAi2o0MT0mqpG7COvP9Uk5fJ8BaO7SZ8OCZVE4Rn7JE wavelabs@wavelabs" \
  --prefix mi-test-vm- \
  --netA 192.168.58.0/24 \
  --netB 192.168.59.0/24 \
  --ip1 197 --ip2 199 \
  --ram-mb 5120 --vcpus 4 --disk-gb 40
```

#### Additional commands
- sudo virsh domiflist mi-test-vm-1
- sudo virsh console mi-test-vm-1
