# Go library for Broadlink RM devices [![pipeline status](https://gitlab.com/waringer/broadlink/badges/master/pipeline.svg)](https://gitlab.com/waringer/broadlink/commits/master) [![Go Report Card](https://goreportcard.com/badge/gitlab.com/waringer/broadlink)](https://goreportcard.com/report/gitlab.com/waringer/broadlink)

This library where created with work from the following pages:

* <https://blog.ipsumdomus.com/broadlink-smart-home-devices-complete-protocol-hack-bc0b4b397af1>
* <https://github.com/mjg59/python-broadlink>
* <https://community.home-assistant.io/t/configuration-of-broadlink-ir-device-and-getting-the-right-ir-codes/48391>

The library is designed to work with Broadlink RM Mini3 or similar devices. It can configure the WLan settings of the device and learn and send IR commands over the device.

**In the root directory is a sample command line program "main.go" where you can see the usage of the library.**

Link to documentation in [![GoDoc](https://godoc.org/gitlab.com/waringer/broadlink/broadlinkrm?status.svg)](https://godoc.org/gitlab.com/waringer/broadlink/broadlinkrm/android)

## How to's

### * Setup a new device

* Put device in "*AP-Mode*"
* Run "*broadlinkrm.Hello*" to find the device (usually it will have the an IP like 192.168.10.1)
* Run "*broadlinkrm.Join*" to let the device join your Wireless LAN

### * Bring the device in "*AP-Mode*"

* Long press the reset button until the blue LED is blinking quickly.
* Long press again until blue LED is blinking slowly.
* Manually connect to the WiFi SSID named *BroadlinkProv*.

## Function's in the library

### Hello

* In:
```timeout time.Duration```,
```deviceIP net.IP```

* Out:
```chan Device```

* Description:
   Find devices and get info's about it.

### Auth

* In:
```dev *Device```

* Description:
   Authenticate against an device. Updates the security info's of the device struct for further usage.

### Command

* In:
```cmd uint32```,
```data []byte```,
```dev *Device```

* Out:
```[]byte```

* Description:
   Send a command to device.

### Join

* In:
```ssid string```,
```password string```,
```securityMode byte```,
```deviceIP net.IP```

* Out:
```[]byte```

* Description:
   Setup a device in AP-mode to use the specified wlan
