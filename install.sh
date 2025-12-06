#!/bin/sh

SN=pin-uploader

set -ex

sudo cp bin/server "/usr/local/bin/$SN"
sudo useradd --system --no-create-home --shell /usr/sbin/nologin --user-group "$SN"
sudo mkdir -p "/var/lib/$SN"
sudo chown -R "$SN:$SN" "/var/lib/$SN"
sudo cp configs/pin-uploader.service /etc/systemd/system/
sudo cp configs/server.example.yaml "/etc/$SN.yaml"

echo "Files installed successfully!"
echo "Please, now edit /etc/$SN.yaml and run 'systemctl start $SN.service'"
