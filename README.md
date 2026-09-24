# Hetzner Storage Backup

This is a simple program to backup, zip, and upload files via sFTP to a Hetzner Storage Box ($4/Mo for 1 TB of space).
It requires just some configuration, the storage box account information, and a Linux machine (bare metal or VPS from any provider).

## Installation

```sh
git clone https://github.com/Reecepbcups/hetzner-storage-backup.git
# git clone git@github.com:Reecepbcups/hetzner-storage-backup.git
cd hetzner-storage-backup/go

# install Go if you don't have it: https://go.dev/doc/install

# builds ./hetzner-backup
just install
# or without just:
# go mod tidy && go build -o hetzner-backup .

# Change your hostname if you want different notification names
hostnamectl set-hostname new-hostname

# copy the default file to the new config file you edit
cp secret.json.example secret.json

# Edit this file with the proper regex files you want to ignore, and folders you want to include.
# NOTE: if you want to match all, ensure you do '.*' as '*' does not work for standard python regex.
# Ex: '.*test/' will ignore 'hello_test/', even though there is no actual period (.) in it.


# BE SURE TO RUN THIS FROM THE ROOT OF WHAT YOU SET 'save-location' IN THE CONFIG
# EDITOR=nano crontab -e
0 2 * * * /root/hetzner-storage-backup/go/hetzner-backup /root/hetzner-storage-backup/go/secret.json

# Times are in UTC (6am UTC = 12am CST)
# Run every night at 2am:            0 2 * * *
# Sunday at 1AM:                     0 1 * * 0
# First day of the month at 3:20PM: 20 15 1 * *
```

## Discord notifications

![image](https://user-images.githubusercontent.com/31943163/209423181-3d78b869-2524-47d1-b32e-48fa20320122.png)
