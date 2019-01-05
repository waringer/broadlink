package main

import (
	"log"

	"gitlab.com/waringer/broadlink/broadlinkrm"
)

func main() {
	log.Println("broadlink-rm test")

	broadlinkrm.DefaultTimeout = 5
	dev := broadlinkrm.Hello(5, nil)
	if len(dev) > 0 {
		log.Printf("Hello: %v \n", dev)

		broadlinkrm.Auth(&dev[0])
		log.Printf("Auth: %v \n", dev[0])

		// send
		// sampleIRCommand, _ := hex.DecodeString("26002400491c1934181c191b1835181c181b181b191c191b181c181c191b181b1835191a19000d0500000000")
		// log.Printf("Command 2: %x \n", broadlinkrm.Command(2, sampleIRCommand, dev[0]))

		// enter learning mode
		// log.Printf("Command 3: %x \n", broadlinkrm.Command(3, nil, dev[0]))
		// time.Sleep(30 * time.Second)

		// get last learned data
		log.Printf("Command 4: %x \n", broadlinkrm.Command(4, nil, &dev[0]))
	} else {
		log.Println("no reponse!")
	}
}
