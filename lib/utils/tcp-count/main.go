// Count the total tcp traffic of a connection through this transparent proxy
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/utils"
)

func main() {
	flag.Parse()

	s, err := net.Listen("tcp4", ":0")
	utils.E(err)

	u := launcher.New().MustLaunch()
	to := regexp.MustCompile(`:\d+`).FindString(u)
	uu, _ := url.Parse(u)
	uu.Host = s.Addr().String()

	fmt.Println("debugging url:", uu.String())

	for {
		conn, err := s.Accept()
		utils.E(err)

		count := int64(0)
		go func() {
			client, err := net.Dial("tcp4", to)
			utils.E(err)

			go func() {
				c, err := io.Copy(conn, client)
				utils.E(err)
				count += c
			}()
			c, err := io.Copy(client, conn)
			utils.E(err)
			fmt.Printf("total traffic: %d bytes\n", count+c)
		}()
	}
}
