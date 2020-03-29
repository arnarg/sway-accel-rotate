package main

import (
	"errors"
	"log"
	"strings"

	"github.com/arnarg/go-iio-sensor-proxy"
	"github.com/godbus/dbus/v5"
)

func SwayRotate(orientation string) (err error) {
	log.Println("Rotating:", orientation)

	var degrees string
	var matrix string

	switch orientation {
	case "normal":
		degrees = "0"
		matrix = "1 0 0 0 1 0"
	case "right-up":
		degrees = "90"
		matrix = "0 1 0 -1 0 1"
	case "bottom-up":
		degrees = "180"
		matrix = "-1 0 1 0 -1 1"
	case "left-up":
		degrees = "270"
		matrix = "0 -1 1 1 0 0"
	default:
		return errors.New("Unrecognized orientation: " + orientation)
	}

	var outputs []string
	outputs, err = GetOutputNames()

	if err != nil {
		return
	}

	for _, output := range outputs {
		err = SwayMsg(nil, "output", output, "transform", degrees)

		if err != nil {
			return
		}
	}

	var inputs []Input
	inputs, err = GetInputs()

	if err != nil {
		return
	}

	for _, input := range inputs {
		if input.Type == "touch" || input.Type == "tablet_tool" {
			err = SwayMsg(nil, "--", "input", input.Identifier, "calibration_matrix", matrix)
			if err != nil {
				return
			}
		}
	}

	return
}

func Claim(sensorProxy sensorproxy.SensorProxy) {
	if err := sensorProxy.ClaimAccelerometer(); err != nil {
		log.Fatal("Failed to claim accelerometer:", err)
	}
}

func Release(sensorProxy sensorproxy.SensorProxy) {
	if err := sensorProxy.ReleaseAccelerometer(); err != nil {
		log.Fatal("Failed to claim accelerometer:", err)
	}
}

func setupWatch(conn *dbus.Conn) error {
	return conn.AddMatchSignal(
		dbus.WithMatchObjectPath("/net/hadess/SensorProxy"),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchSender("net.hadess.SensorProxy"),
		dbus.WithMatchMember("PropertiesChanged"),
	)
}

func watchCurrentOrientation(conn *dbus.Conn, orientCh chan<- string) {
	c := make(chan *dbus.Signal, 10)
	conn.Signal(c)

	for v := range c {
		message := v.Body[1].(map[string]dbus.Variant)
		orientation, ok := message["AccelerometerOrientation"]
		if ok {
			orientCh <- strings.ReplaceAll(orientation.String(), "\"", "")
		}
	}
}

func main() {
	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatal("Failed to connect to system bus:", err)
	}

	sensorProxy, err := sensorproxy.NewSensorProxyFromBus(conn)
	if err != nil {
		log.Fatal("Failed to get sensorProxy object from DBus:", err)
	}

	hasAccel, err := sensorProxy.HasAccelerometer()
	if err != nil {
		log.Fatal("Failed to check whether device has an accelerometer:", err)
	}
	if !hasAccel {
		log.Fatal("No accelerometer found")
	}

	err = setupWatch(conn)
	if err != nil {
		log.Fatal("Failed setting up watch for orientation.")
	}

	orientCh := make(chan string)
	go watchCurrentOrientation(conn, orientCh)

	Claim(sensorProxy)
	defer Release(sensorProxy)

	for orientation := range orientCh {
		err = SwayRotate(orientation)

		if err != nil {
			log.Fatal("Unable to rotate:", err)
		}
	}
}
