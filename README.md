# Go library for Broadlink RM devices [![pipeline status](https://gitlab.com/waringer/broadlink/badges/master/pipeline.svg)](https://gitlab.com/waringer/broadlink/commits/master)

This library where created with work from the following pages:
* https://blog.ipsumdomus.com/broadlink-smart-home-devices-complete-protocol-hack-bc0b4b397af1
* https://github.com/mjg59/python-broadlink
* https://community.home-assistant.io/t/configuration-of-broadlink-ir-device-and-getting-the-right-ir-codes/48391

The library is designed to work with a Broadlink RM Mini3 or similar devices. It can configure the WLan settings of the device and learn and send IR commands over the device.

**In the root directory is sample command line program "main.go" where you can see the usage of the library.**

## How to's
### * Setup a new device
* Put device in "*AP-Mode*"
* Run "*broadlinkrm.Hello*" to find the device (usually it will have the an IP like 192.168.x.x)
* Run "*broadlinkrm.Join*" to let the device join your Wireless LAN

### * Bring the device in "*AP-Mode*"
* Long press the reset button until the blue LED is blinking quickly.
* Long press again until blue LED is blinking slowly.
* Manually connect to the WiFi SSID named *BroadlinkProv*.

