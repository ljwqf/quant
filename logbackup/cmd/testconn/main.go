package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	conn, err := net.DialTimeout("tcp", "132.232.231.41:8600", 5*time.Second)
	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		return
	}
	defer conn.Close()
	fmt.Println("Connected to 132.232.231.41:8600 successfully!")
}
