package main

import (
	"log"
	"time"
)

func main() {
	log.Println("HEYYY")
	for {
		time.Sleep(10 * time.Second)
		log.Println("Still here!")
	}
}
