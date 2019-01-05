package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"gitlab.com/waringer/broadlink/broadlinkrm"
)

func main() {
	fmt.Println("Broadlink RM Toolset")

	deviceIP := flag.String("ip", "", "ip of device")
	cmdDiscover := flag.Bool("discover", false, "search for devices")
	cmdAuth := flag.Bool("auth", true, "authenticate agaist device")
	flag.Parse()

	broadlinkrm.DefaultTimeout = 5

	var (
		ip  net.IP
		dev []broadlinkrm.Device
	)
	if *deviceIP != "" {
		ip = net.ParseIP(*deviceIP)
	}

	if *cmdDiscover {
		if ip == nil {
			dev = broadlinkrm.Hello(5, nil)
		} else {
			dev = broadlinkrm.Hello(0, ip)
		}

		log.Printf("Found %v devices\n", len(dev))
		for id, device := range dev {
			fmt.Printf("[%02v] Device type: %X \n", id, device.DeviceType)
			fmt.Printf("[%02v] Device name: %v \n", id, device.DeviceName)
			fmt.Printf("[%02v] Device MAC: [% x] \n", id, device.DeviceMac)
			fmt.Printf("[%02v] Device IP: %v \n", id, device.DeviceAddr.IP)
		}
	}

	if *cmdAuth {
		for id, device := range dev {
			broadlinkrm.Auth(&device)
			fmt.Printf("[%02v] Device authenticated \n", id)
		}
	}

	// send
	// sampleIRCommand, _ := hex.DecodeString("26002400491c1934181c191b1835181c181b181b191c191b181c181c191b181b1835191a19000d0500000000")
	// log.Printf("Command 2: %x \n", broadlinkrm.Command(2, sampleIRCommand, dev[0]))

	// enter learning mode
	// log.Printf("Command 3: %x \n", broadlinkrm.Command(3, nil, dev[0]))
	// time.Sleep(30 * time.Second)

	// get last learned data
	// 	log.Printf("Command 4: %x \n", broadlinkrm.Command(4, nil, &dev[0]))
}
