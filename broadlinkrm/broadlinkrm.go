// Package broadlinkrm is designed to work with Broadlink RM Mini3 or similar devices.
// It can configure the WLan settings of the device and learn and send IR commands over the device.
package broadlinkrm

/* Copyright (c) 2019, Holger Wolff - All rights reserved.
   Licenced under BSD 3-Clause License */

// Build with help from:
// https://blog.ipsumdomus.com/broadlink-smart-home-devices-complete-protocol-hack-bc0b4b397af1
// https://github.com/mjg59/python-broadlink

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"log"
	"math"
	"net"
	"os"
	"time"
)

// Device holds all info about an device
type Device struct {
	DeviceType uint16
	DeviceName string
	DeviceAddr *net.UDPAddr
	deviceMac  [6]byte
	deviceID   uint32
	deviceKey  []byte
}

// DeviceMac gets the mac of the device
func (dev Device) DeviceMac() []byte {
	return reverseArray(dev.deviceMac[:])
}

const (
	broadcast = "255.255.255.255:80"
)

var (
	// DefaultTimeout to use for waiting for response
	DefaultTimeout = time.Duration(60)
	// LogWarnings - set to true to see warnings in log
	LogWarnings = false

	defaultKey       = []byte{0x09, 0x76, 0x28, 0x34, 0x3f, 0xe9, 0x9e, 0x23, 0x76, 0x5c, 0x15, 0x13, 0xac, 0xcf, 0x8b, 0x02}
	deviceIv         = []byte{0x56, 0x2e, 0x17, 0x99, 0x6d, 0x09, 0x3d, 0x28, 0xdd, 0xb3, 0xba, 0x69, 0x5a, 0x2e, 0x6f, 0x58}
	sendCount        = uint16(0)
	udpServer, _     = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	responses        chan ([]byte)
	updBroadcastAddr *net.UDPAddr
)

func init() {
	if responses == nil {
		updBroadcastAddr, _ = net.ResolveUDPAddr("udp4", broadcast)
		responses = make(chan []byte, 1000)
		go udpListener()
	}
}

// Hello - find broadlink devices on the local network and get base infos about the devices.
// Devices in AP-Mode do not respond to Hello messages.
//
// timeout - in seconds, if set to 0 the function returns after the first device that answers
// deviceIP - IP of an device to use, if nil a broadcast will be send to find all devices
// The returned channel contains the parsed data of the devices that have answered.
func Hello(timeout time.Duration, deviceIP net.IP) (devices chan Device) {
	payload := make([]byte, 0x30)

	binary.LittleEndian.PutUint16(payload[0x0c:], uint16(time.Now().UTC().Year()))
	payload[0x0e] = byte(time.Now().UTC().Minute())
	payload[0x0f] = byte(time.Now().UTC().Hour())
	payload[0x10] = byte(time.Now().UTC().Year() - 2000)
	payload[0x11] = byte(time.Now().UTC().Weekday())
	payload[0x12] = byte(time.Now().UTC().Day())
	payload[0x13] = byte(time.Now().UTC().Month())
	copy(payload[0x18:0x1c], getLocalIP())                                                           // unused by device - answers to origin ip!
	binary.LittleEndian.PutUint16(payload[0x1c:], uint16(udpServer.LocalAddr().(*net.UDPAddr).Port)) // unused by device - answers to origin port!
	payload[0x26] = 0x06                                                                             // Command Hello
	binary.LittleEndian.PutUint16(payload[0x20:], makeChecksum(payload))

	if deviceIP == nil {
		udpServer.WriteTo(payload, updBroadcastAddr)
	} else {
		udpServer.WriteTo(payload, &net.UDPAddr{IP: deviceIP, Port: 80})
	}

	devices = make(chan (Device), 100)

	go asyncHelloResponse(timeout, devices)

	return
}

func asyncHelloResponse(timeout time.Duration, devices chan Device) {
	defer close(devices)

	startTime := time.Now().Add(timeout * time.Second)
	waitTimeout := timeout

	if timeout < 1 {
		startTime = time.Now().Add(DefaultTimeout * time.Second)
		waitTimeout = DefaultTimeout
	}

	for {
		buf := wait4Response(0x07, waitTimeout)

		if buf != nil {
			dev := Device{
				DeviceType: binary.LittleEndian.Uint16(buf[0x34:]),
				DeviceName: string(buf[0x40:]),
				deviceID:   0,
				deviceKey:  make([]byte, len(defaultKey)),
			}

			copy(dev.deviceMac[:], buf[0x3a:0x40])
			copy(dev.deviceKey, defaultKey)
			dev.DeviceAddr = &net.UDPAddr{IP: net.IPv4(buf[0x39], buf[0x38], buf[0x37], buf[0x36]), Port: 80}
			devices <- dev
		} else if !time.Now().Before(startTime) {
			break
		}

		if timeout == 0 {
			break
		}
	}
}

// Command - send a command with parameters to an device.
//
// cmd - command to send, knowen command's: 2 send, 3 learn, 4 fetch last learned code
// data - parameters for command
// dev - device structure returned from Hello where command is send to
// Returned are the decrypted raw answer from the device
func Command(cmd uint32, data []byte, dev *Device) []byte {
	var payload []byte
	if (data == nil) || (len(data) < 12) {
		payload = make([]byte, 16)
		copy(payload[4:], data)
	} else {
		payload = make([]byte, 4)
		payload = append(payload, data...)
	}

	binary.LittleEndian.PutUint32(payload[0x00:], cmd)

	if dev.DeviceType == 0x5f36 {
		payload = append([]byte{0x04, 0x00}, payload...)
	}

	send(0x6a, dev, payload)

	response := wait4Response(0x3ee, DefaultTimeout)
	if len(response) >= 0x22 {
		if int16(binary.LittleEndian.Uint16(response[0x22:])) == 0 {
			decrypted, _ := decrypt(dev.deviceKey, deviceIv, response[0x38:])
			return decrypted[4:]
		}

		if LogWarnings {
			log.Println("error packet received")
		}
	} else {
		log.Printf("packet received is to small (%d) %v\n", len(response), response)
	}

	return nil
}

// Join a wireless network. Device needs to be in AP-Mode.
//
// ssid - name of the wireless network
// password - password of the wireless network
// securityModes - protection mode of the wireless network, possible knowen modes are: 0=none, 1=wep, 2=wpa1, 3=wpa2, 4=wpa1/2 CCMP, 6=wpa1/2 TKIP
// deviceIP - IP of an device to use, if nil a broadcast will be send
// Returned are the raw answer from the device.
func Join(ssid string, password string, securityMode byte, deviceIP net.IP) []byte {
	payload := make([]byte, 0x88)

	payload[0x26] = 0x14 // Command Join

	copy(payload[0x44:], []byte(ssid))
	copy(payload[0x64:], []byte(password))

	payload[0x84] = byte(len(ssid))
	payload[0x85] = byte(len(password))
	payload[0x86] = securityMode

	binary.LittleEndian.PutUint16(payload[0x20:0x22], makeChecksum(payload))

	if deviceIP == nil {
		udpServer.WriteTo(payload, updBroadcastAddr)
	} else {
		udpServer.WriteTo(payload, &net.UDPAddr{IP: deviceIP, Port: 80})
	}

	response := wait4Response(0x15, DefaultTimeout)

	// todo
	// expected response 0000000000000000000000000000000000000000000000000000000000000000c4be0000000015000000000000000000
	// check it
	return response
}

// Auth against an device for further usage.
//
// dev - device structure returned from Hello where authentication is send to
func Auth(dev *Device) {
	payload := make([]byte, 0x50)
	payload[0x2d] = 0x01

	hostname, _ := os.Hostname()
	copy(payload[0x30:], []byte(hostname))

	send(0x65, dev, payload)

	decrypted, _ := decrypt(dev.deviceKey, deviceIv, wait4Response(0x3e9, DefaultTimeout)[0x38:])
	dev.deviceID = binary.LittleEndian.Uint32(decrypted[0x00:])
	dev.deviceKey = decrypted[0x04:0x14]
}

func makeChecksum(payload []byte) uint16 {
	checksum := uint16(0xbeaf)

	for _, val := range payload {
		checksum += uint16(val)
	}

	return checksum
}

func checkChecksum(payload []byte, checksumPos int) bool {
	origChecksum := binary.LittleEndian.Uint16(payload[checksumPos : checksumPos+2])
	binary.LittleEndian.PutUint16(payload[checksumPos:checksumPos+2], 0)

	newChecksum := makeChecksum(payload)

	binary.LittleEndian.PutUint16(payload[checksumPos:checksumPos+2], origChecksum)

	return newChecksum == origChecksum
}

func reverseArray(a []byte) []byte {
	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}

	return a
}

func getLocalIP() []byte {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.To4()
			}
		}
	}
	return nil
}

func encrypt(key, iv, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := padding(text, aes.BlockSize)
	ciphertext := make([]byte, len(b))

	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, b)

	return ciphertext, nil
}

func decrypt(key []byte, iv []byte, encText []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(encText) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	decrypted := make([]byte, len(encText))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(decrypted, encText)

	return unPadding(decrypted), nil
}

func padding(ciphertext []byte, blockSize int) []byte {
	return append(ciphertext, bytes.Repeat([]byte{0x00}, blockSize-len(ciphertext)%blockSize)...)
}

func unPadding(src []byte) []byte {
	length := len(src)
	unpadding := int(src[length-1])
	return src[:(length - unpadding)]
}

func udpListener() {
	for {
		buf := make([]byte, 2048)
		count, _, err := udpServer.ReadFrom(buf)
		if err != nil {
			continue
		}

		if checkChecksum(buf, 0x20) {
			response := make([]byte, count)
			copy(response, buf)
			responses <- response
		}
	}
}

func wait4Response(expectedType uint16, timeout time.Duration) []byte {
	startTime := time.Now().Add(timeout * time.Second)
	for {
		select {
		case buf := <-responses:
			msgType := binary.LittleEndian.Uint16(buf[0x26:0x28])
			if msgType == expectedType {
				return buf
			}

			responses <- buf
			if !time.Now().Before(startTime) {
				return nil
			}
		case <-time.After(timeout * time.Second):
			return nil
		}
	}
}

func send(command uint16, dev *Device, payload []byte) {
	sendCount++

	buffer := make([]byte, 0x38)
	copy(buffer[0:], []byte{0x5a, 0xa5, 0xaa, 0x55, 0x5a, 0xa5, 0xaa, 0x55, 0x00})
	binary.LittleEndian.PutUint16(buffer[0x24:], dev.DeviceType)
	binary.LittleEndian.PutUint16(buffer[0x26:], command)
	binary.LittleEndian.PutUint16(buffer[0x28:], sendCount)
	copy(buffer[0x2a:], dev.deviceMac[0:])
	binary.LittleEndian.PutUint32(buffer[0x30:], dev.deviceID)
	if (payload != nil) && (len(payload) > 0) {
		binary.LittleEndian.PutUint16(buffer[0x34:], makeChecksum(payload))
		encrypted, _ := encrypt(dev.deviceKey, deviceIv, payload)
		buffer = append(buffer, encrypted...)
	}

	binary.LittleEndian.PutUint16(buffer[0x20:], makeChecksum(buffer))

	udpServer.WriteToUDP(buffer, dev.DeviceAddr)
}

// *** Converter ***
// Based on the code from https://community.home-assistant.io/t/configuration-of-broadlink-ir-device-and-getting-the-right-ir-codes/48391

// ConvertPronto2Broadlink converts pronto codes to broadlink code
func ConvertPronto2Broadlink(prontoByte []byte) []byte {
	lircCode := pronto2lirc(prontoByte)
	broadlinkCode := lirc2broadlink(lircCode)

	return broadlinkCode
}

// ConvertBroadlink2Pronto converts broadlink codes to pronto code
func ConvertBroadlink2Pronto(broadlinkByte []byte, prontoIrFrequency uint16) []byte {
	lircCode := broadlink2lirc(broadlinkByte)
	prontoByte := lirc2pronto(lircCode, prontoIrFrequency)

	return prontoByte
}

func pronto2lirc(prontoCode []byte) []int {
	codes := make([]uint16, len(prontoCode)/2)

	for i := 0; i < len(prontoCode); i = i + 2 {
		codes[i/2] = binary.BigEndian.Uint16(prontoCode[i : i+2])
	}

	if codes[0] != 0 {
		log.Fatal("Pronto code should start with 0000")
	}

	if uint32(len(codes)) != 4+2*(binary.BigEndian.Uint32(prontoCode[4:])) {
		log.Fatal("Number of pulse widths does not match the preamble")
	}

	frequency := 1 / (float64(codes[1]) * 0.241246)
	lircCode := make([]int, len(codes)-4)
	for i := 4; i < len(codes); i++ {
		lircCode[i-4] = int(math.Round(float64(codes[i]) / frequency))
	}

	return lircCode
}

func lirc2broadlink(lircCode []int) []byte {
	var pulses []byte

	for i := 0; i < len(lircCode); i++ {
		pulse := lircCode[i] * 269 / 8192

		if pulse < 256 {
			pulses = append(pulses, byte(pulse))
		} else {
			pulses = append(pulses, byte(0))
			pulseByte := make([]byte, 2)
			binary.BigEndian.PutUint16(pulseByte, uint16(pulse))
			pulses = append(pulses, pulseByte[0], pulseByte[1])
		}
	}

	var packet []byte
	packet = append(packet, 0x26, 0x00)
	packetLen := make([]byte, 2)
	binary.LittleEndian.PutUint16(packetLen, uint16(len(pulses)))
	packet = append(packet, packetLen[0], packetLen[1])
	packet = append(packet, pulses...)
	packet = append(packet, 0x0d, 0x05)

	return packet
}

func broadlink2lirc(broadlinkCode []byte) []int {
	if uint16(broadlinkCode[0]+broadlinkCode[1]) != 0x26 {
		log.Fatal("Broadlink code do not start with 0x26 0x00")
	}

	pulseLen := binary.LittleEndian.Uint16(broadlinkCode[2:4])
	pulses := broadlinkCode[4 : pulseLen+4]
	var lircCode []int

	for i := 0; i < len(pulses); i++ {
		if pulses[i] == 0 {
			i++
			pulse := int(binary.BigEndian.Uint16(pulses[i:i+2])) * 8192 / 269
			lircCode = append(lircCode, pulse)
			i++
		} else {
			pulse := int(byte(pulses[i])) * 8192 / 269
			lircCode = append(lircCode, pulse)
		}
	}

	return lircCode
}

func lirc2pronto(lircCode []int, ff uint16) []byte {
	frequency := 1 / (float64(ff) * 0.241246)

	var prontoByte []uint16
	prontoByte = append(prontoByte, uint16(0))
	prontoByte = append(prontoByte, ff)
	prontoByte = append(prontoByte, uint16(0))
	prontoByte = append(prontoByte, uint16(33))
	for i := 0; i < len(lircCode); i++ {
		prontoByte = append(prontoByte, uint16(math.Round(float64(lircCode[i])*frequency)))
	}
	prontoByte[3] = uint16(len(prontoByte)/2 - 2)

	var prontoCode []byte
	for i := 0; i < len(prontoByte); i++ {
		beB := make([]byte, 2)
		binary.BigEndian.PutUint16(beB, prontoByte[i])
		prontoCode = append(prontoCode, beB...)
	}

	return prontoCode
}
