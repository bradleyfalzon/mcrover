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
	"net/http"
	"strconv"
	"strings"

	"github.com/d2r2/go-i2c"
	"github.com/gorilla/websocket"
)

var (
	pan  int = 375
	tilt int = 300
	pwm  *i2c.I2C
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
		log.Printf("received: %s\n", p)
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

		log.Printf("ws type: %v, msg: %s", messageType, p)
		if err := conn.WriteMessage(messageType, p); err != nil {
			log.Println(err)
			return
		}
	}

}
