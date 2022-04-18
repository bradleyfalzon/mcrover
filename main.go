/*

A PWM example by @Drahoslav7, using the go-rpio library

Toggles a LED on physical pin 35 (GPIO pin 19)
Connect a LED with resistor from pin 35 to ground.

*/

package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/d2r2/go-i2c"
	"github.com/gorilla/websocket"
	"github.com/stianeikeland/go-rpio"
)

var (
	pan  int = 375
	tilt int = 300
	pwm  *i2c.I2C

	lhsForward,
	lhsReverse,
	rhsForward,
	rhsReverse rpio.Pin
)

const (
	LHS_Forward_Pin = 24
	LHS_Reverse_Pin = 23
	LHS_PWM_Addr    = 0x42
	RHS_Forward_Pin = 17
	RHS_Reverse_Pin = 22
	RHS_PWM_Addr    = 0x3e
)

func main() {

	var err error
	pwm, err = i2c.NewI2C(0x40, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer pwm.Close()

	buf, n, err := pwm.ReadRegBytes(0x00, 1)
	if err != nil {
		log.Fatal(err)
	}
	if n != 1 {
		log.Fatalf("failed to read bytes, have %d, want %d", n, 1)
	}
	log.Printf("%x\n", buf)

	bs := make([]byte, 2)
	binary.LittleEndian.PutUint16(bs, uint16(pan))
	_, err = pwm.WriteBytes(append([]byte{0x06, 0x00, 0x00}, bs...))
	if err != nil {
		log.Fatal("could not write to i2c pwm:", err)
	}

	binary.LittleEndian.PutUint16(bs, uint16(tilt))
	_, err = pwm.WriteBytes(append([]byte{0x0a, 0x00, 0x00}, bs...))
	if err != nil {
		log.Fatal("could not write to i2c pwm:", err)
	}

	// Configure GPIO
	if err := rpio.Open(); err != nil {
		log.Fatal("could not open raspberry pi gpio:", err)
	}
	lhsForward = rpio.Pin(LHS_Forward_Pin)
	lhsForward.PullDown()
	lhsForward.Output()
	lhsForward.Low()

	lhsReverse = rpio.Pin(LHS_Reverse_Pin)
	lhsReverse.PullDown()
	lhsReverse.Output()
	lhsReverse.Low()

	rhsForward = rpio.Pin(RHS_Forward_Pin)
	rhsForward.PullDown()
	rhsForward.Output()
	rhsForward.Low()

	rhsReverse = rpio.Pin(RHS_Reverse_Pin)
	rhsReverse.PullDown()
	rhsReverse.Output()
	rhsReverse.Low()

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static", fs))
	http.HandleFunc("/ws", serveWS)
	http.HandleFunc("/", serveIndex)

	fmt.Printf("Starting server at port 3000\n")
	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatal(err)
	}

}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.ServeFile(w, r, "index.html")
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		s := strings.Split(string(p), ",")
		log.Printf("received: %s %+v\n", p, s)
		if strings.HasPrefix(s[0], "cameraPan") {
			v, err := strconv.ParseFloat(s[1], 32)
			if err != nil {
				log.Printf("cannot convert float value for command '%s': %s", p, err)
			}

			newPan := pan - int(v*10)
			if newPan < 150 {
				newPan = 150
			} else if newPan > 600 {
				newPan = 600
			}

			log.Printf("old pan: %v, new pan: %v", pan, newPan)

			bs := make([]byte, 2)
			binary.LittleEndian.PutUint16(bs, uint16(newPan))
			_, err = pwm.WriteBytes(append([]byte{0x06, 0x00, 0x00}, bs...))
			if err != nil {
				log.Println("could not write new pan value to pwm:", err)
				continue
			}

			pan = newPan
		}

		if strings.HasPrefix(s[0], "cameraTilt") {
			v, err := strconv.ParseFloat(s[1], 32)
			if err != nil {
				log.Printf("cannot convert float value for command '%s': %s", p, err)
			}

			newTilt := tilt + int(v*10)
			if newTilt < 150 {
				newTilt = 150
			} else if newTilt > 600 {
				newTilt = 600
			}

			log.Printf("old tilt: %v, new tilt: %v", tilt, newTilt)

			bs := make([]byte, 2)
			binary.LittleEndian.PutUint16(bs, uint16(newTilt))
			_, err = pwm.WriteBytes(append([]byte{0x0a, 0x00, 0x00}, bs...))
			if err != nil {
				log.Println("could not write new tilt value to pwm:", err)
				continue
			}

			tilt = newTilt
		}

		if strings.HasPrefix(s[0], "move") {
			xaxis, err := strconv.ParseFloat(s[1], 32)
			if err != nil {
				log.Printf("cannot convert x-axis float value for command '%s': %s", p, err)
			}
			_ = xaxis

			yaxis, err := strconv.ParseFloat(s[2], 32)
			if err != nil {
				log.Printf("cannot convert y-axis float value for command '%s': %s", p, err)
			}

			lhsForwardState := rpio.Low
			lhsReverseState := rpio.Low
			rhsForwardState := rpio.Low
			rhsReverseState := rpio.Low

			// Direction: forward, reverse or stop
			if yaxis == 0 {
				// Stop
			}

			if yaxis < 0 {
				lhsForwardState = rpio.High
				rhsForwardState = rpio.High
			}

			if yaxis > 0 {
				lhsReverseState = rpio.High
				rhsReverseState = rpio.High
			}

			// Speed
			yaxisPower := math.Abs(yaxis * (1 - math.Pow(2, 12)))
			xaxisPower := math.Abs(xaxis * (1 - math.Pow(2, 12)))

			lhsPwmVal := math.Max(yaxisPower, xaxisPower)
			rhsPwmVal := math.Max(yaxisPower, xaxisPower)

			// Differential
			if xaxis > 0.3 {
				// Turning right
				rhsPwmBias := 1 - math.Abs(xaxis)*2
				log.Printf("turning right: old rhsPwm: %.0f, rhsPwmBias: %.2f, new rhsPwm: %.0f", rhsPwmVal, rhsPwmBias, rhsPwmVal*rhsPwmBias)
				rhsPwmVal = rhsPwmVal * rhsPwmBias

				//rhsPwmVal = 0
				//rhsForward.Low()
				//rhsReverse.Low()

				if rhsPwmVal < 0 {
					rhsForwardState = rpio.Low
					rhsReverseState = rpio.High
					rhsPwmVal = math.Abs(rhsPwmVal)
				}

			}

			if xaxis < -0.3 {
				//if xaxis < 0 && xaxis > -0.5 {
				// Turning left
				lhsPwmBias := 1 - math.Abs(xaxis)*2
				log.Printf("turning left: old lhsPwm: %.0f, lhsPwmBias: %.2f, new lhsPwm: %.0f", lhsPwmVal, lhsPwmBias, lhsPwmVal*lhsPwmBias)
				lhsPwmVal = lhsPwmVal * lhsPwmBias

				//lhsPwmVal = 0
				//lhsForward.Low()
				//lhsReverse.Low()

				if lhsPwmVal < 0 {
					lhsForwardState = rpio.Low
					lhsReverseState = rpio.High
					lhsPwmVal = math.Abs(lhsPwmVal)
				}
			}

			lhsPwmVal = math.Max(lhsPwmVal, 0)
			lhsPwmVal = math.Min(lhsPwmVal, math.Pow(2, 12))

			rhsPwmVal = math.Max(rhsPwmVal, 0)
			rhsPwmVal = math.Min(rhsPwmVal, math.Pow(2, 12))

			log.Printf("x: %.0f, y: %.0f, lhs pwm: %.0f, rhs pwm: %.0f\n", xaxis, yaxis, lhsPwmVal, rhsPwmVal)

			// LHS
			bs := make([]byte, 2)
			binary.LittleEndian.PutUint16(bs, uint16(rhsPwmVal))
			_, err = pwm.WriteBytes(append([]byte{LHS_PWM_Addr, 0x00, 0x00}, bs...))
			if err != nil {
				log.Println("could not write new pwm speed:", err)
			}
			lhsForward.Write(lhsForwardState)
			lhsReverse.Write(lhsReverseState)

			// RHS
			binary.LittleEndian.PutUint16(bs, uint16(rhsPwmVal))
			_, err = pwm.WriteBytes(append([]byte{RHS_PWM_Addr, 0x00, 0x00}, bs...))
			if err != nil {
				log.Println("could not write new pwm speed:", err)
			}
			rhsForward.Write(rhsForwardState)
			rhsReverse.Write(rhsReverseState)
		}

		log.Printf("ws type: %v, msg: %s", messageType, p)
		if err := conn.WriteMessage(messageType, p); err != nil {
			log.Println(err)
			return
		}
	}

}
