package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/waringer/broadlink/broadlinkrm"
)

type cmdArguments struct {
	cmdConvertBroadlink *string
	cmdConvertPronto    *string
	deviceIP            *string
	cmdSend             *string
	cmdSendPronto       *string
	setupPassword       *string
	setupSSID           *string

	setupSecurity *uint

	cmdAuth       *bool
	cmdDiscover   *bool
	cmdLearn      *bool
	cmdGetLearned *bool
	cmdQuiet      *bool
	cmdSetup      *bool
	cmdVerbose    *bool
}

var logLevel = 1

func main() {
	args := getArguments()
	checkArguments(args)

	broadlinkrm.DefaultTimeout = 5
	broadlinkrm.LogWarnings = *args.cmdVerbose

	if *args.cmdVerbose {
		logLevel++
	}

	if *args.cmdQuiet {
		logLevel = 0
	}

	printMessage(1, fmt.Sprintf("Broadlink RM Toolbox\n"))

	convertBroadlink(*args.cmdConvertBroadlink)
	convertPronto(*args.cmdConvertPronto)

	ip := net.ParseIP(*args.deviceIP)

	if *args.cmdDiscover {
		dev := discover(ip, *args.cmdAuth)
		learn(*args.cmdLearn, *args.cmdGetLearned, dev)
		send(buildIRcommand(*args.cmdSend, *args.cmdSendPronto), dev)
	}

	if *args.cmdSetup {
		response := broadlinkrm.Join(*args.setupSSID, *args.setupPassword, byte(*args.setupSecurity), ip)
		printMessage(1, fmt.Sprintf("Device returned: [%x] \n", response))
	}
}

func getArguments() (args cmdArguments) {
	args.cmdAuth = flag.Bool("a", true, "authenticate agaist device")
	args.cmdConvertBroadlink = flag.String("convertbroadlink", "", "convert code provided in Broadlink format to Pronto format")
	args.cmdConvertPronto = flag.String("convertpronto", "", "convert code provided in Pronto format to Broadlink format")
	args.cmdDiscover = flag.Bool("d", false, "discover - search for devices")
	args.deviceIP = flag.String("ip", "", "ip of device")
	args.cmdLearn = flag.Bool("learn", false, "put device in learing mode and wait up to 30 seconds for new learned code")
	args.cmdGetLearned = flag.Bool("learned", false, "get the last learned code from device in Broadlink format")
	args.cmdQuiet = flag.Bool("q", false, "quiet - only errors may showen")
	args.cmdSend = flag.String("send", "", "send code provided in Broadlink format")
	args.cmdSendPronto = flag.String("sendpronto", "", "send code provided in Pronto format")

	args.cmdSetup = flag.Bool("setup", false, "set device wlan settings - device needs to be in AP-Mode for this")
	args.setupPassword = flag.String("setuppassword", "", "password of wlan for the device setup")
	args.setupSecurity = flag.Uint("setupsecurity", 0, "type of wlan security for the device setup [0-none, 1-wep, 2-wpa1, 3-wpa2]")
	args.setupSSID = flag.String("setupssid", "", "ssid of wlan for the device setup")

	args.cmdVerbose = flag.Bool("v", false, "verbose - show detailed messages")
	flag.Parse()
	return
}

func checkArguments(args cmdArguments) {
	if (*args.cmdLearn || (len(*args.cmdSend) != 0) || (len(*args.cmdSendPronto) != 0) || *args.cmdGetLearned) && !*args.cmdDiscover {
		log.Fatalln("invalid options - discovery needed")
	}

	if *args.cmdSetup {
		if len(*args.setupSSID) == 0 {
			log.Fatalln("No SSID provided")
		}

		if (*args.setupSecurity != 0) && (len(*args.setupPassword) == 0) {
			log.Fatalln("No WLan Password provided")
		}

		if *args.setupSecurity > 3 {
			log.Fatalln("Unsupported WLan security type")
		}
	}
}

func convertBroadlink(cmdConvertBroadlink string) {
	if len(cmdConvertBroadlink) != 0 {
		broadlinkCode, errBroadlink := hex.DecodeString(strings.Replace(cmdConvertBroadlink, " ", "", -1))

		if errBroadlink != nil {
			log.Fatalln("Provided Broadlink IR code is invalid")
		}

		printMessage(0, fmt.Sprintf("Converted IR code in Pronto format: %v \n", regexp.MustCompile("(?m)(.{4})").ReplaceAllString(hex.EncodeToString(broadlinkrm.ConvertBroadlink2Pronto(broadlinkCode, 0x6d)), "$1 ")))
	}
}

func convertPronto(cmdConvertPronto string) {
	if len(cmdConvertPronto) != 0 {
		prontoCode, errPronto := hex.DecodeString(strings.Replace(cmdConvertPronto, " ", "", -1))

		if errPronto != nil {
			log.Fatalln("Provided Pronto IR code is invalid")
		}

		printMessage(0, fmt.Sprintf("Converted IR code in Broadlink format: %x \n", broadlinkrm.ConvertPronto2Broadlink(prontoCode)))
	}
}

func discover(ip net.IP, cmdAuth bool) (dev []broadlinkrm.Device) {
	var devC chan (broadlinkrm.Device)

	if ip == nil {
		devC = broadlinkrm.Hello(5, nil)
	} else {
		devC = broadlinkrm.Hello(0, ip)
	}

	id := 0
	for device := range devC {
		id++

		printMessage(2, fmt.Sprintf("[%02v] Device type: %X \n", id, device.DeviceType))
		printMessage(2, fmt.Sprintf("[%02v] Device name: %v \n", id, device.DeviceName))
		printMessage(2, fmt.Sprintf("[%02v] Device MAC: [% x] \n", id, device.DeviceMac()))
		printMessage(1, fmt.Sprintf("[%02v] Device IP: %v \n", id, device.DeviceAddr.IP))

		if cmdAuth {
			broadlinkrm.Auth(&device)
			printMessage(2, fmt.Sprintf("[%02v] Device authenticated \n", id))
		}

		dev = append(dev, device)
	}

	printMessage(1, fmt.Sprintf("Found %v device(s)\n", len(dev)))
	return
}

func learn(cmdLearn bool, cmdGetLearned bool, dev []broadlinkrm.Device) {
	if cmdLearn {
		for id, device := range dev {
			broadlinkrm.Command(3, nil, &device)
			printMessage(0, fmt.Sprintf("[%02v] Wait for learned code", id))

			var learnedCode []byte
			startTime := time.Now().Add(30 * time.Second)
			for time.Now().Before(startTime) {
				learnedCode = broadlinkrm.Command(4, nil, &device)

				if len(learnedCode) != 0 {
					printMessage(0, fmt.Sprintf("\n[%02v] Learned code: [%x] \n", id, learnedCode))
					break
				}
				fmt.Print(".")
				time.Sleep(1 * time.Second)
			}

			if learnedCode == nil {
				printMessage(0, fmt.Sprintf("\n[%02v] No code learned! \n", id))
			}
		}
	} else if cmdGetLearned {
		for id, device := range dev {
			learnedCode := broadlinkrm.Command(4, nil, &device)
			printMessage(0, fmt.Sprintf("[%02v] Device last learned code: [%x] \n", id, learnedCode))
		}
	}
}

func buildIRcommand(cmdSend string, cmdSendPronto string) (irCommand []byte) {
	var err error
	if len(cmdSend) != 0 {
		irCommand, err = hex.DecodeString(strings.Replace(cmdSend, " ", "", -1))

		if err != nil {
			log.Fatalln("Provided Broadlink IR code is invalid")
		}
	} else if len(cmdSendPronto) != 0 {
		irCommand, err = hex.DecodeString(strings.Replace(cmdSendPronto, " ", "", -1))

		if err != nil {
			log.Fatalln("Provided Pronto IR code is invalid")
		}

		irCommand = broadlinkrm.ConvertPronto2Broadlink(irCommand)
	}

	return
}

func send(irCommand []byte, dev []broadlinkrm.Device) {
	if irCommand != nil {
		for id, device := range dev {
			response := broadlinkrm.Command(2, irCommand, &device)

			if response == nil {
				printMessage(0, fmt.Sprintf("[%02v] code send failed!\n", id))
			} else {
				printMessage(1, fmt.Sprintf("[%02v] code send \n", id))
			}
		}
	}
}

func printMessage(level int, message string) {
	if logLevel >= level {
		fmt.Print(message)
	}
}
