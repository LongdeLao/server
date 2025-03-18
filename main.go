package main

import (
	"fmt"
	"log"
	"net"
	"server/routes" // Adjust the import path based on your module

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()
	
	// Register login route (and others as needed)
	routes.RegisterLoginRoute(router)
	// routes.RegisterOtherRoutes(router)

	// Print local non-loopback IPv4 addresses
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Error getting IP addresses: %v", err)
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				fmt.Printf("Running on IP: %s\n", ip4.String())
			}
		}
	}

	// Start the server on port 2000
	if err := router.Run(":2000"); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
