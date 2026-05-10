package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services/database"
)

func handleRegisterAdmin() {
	fs := flag.NewFlagSet("register-admin", flag.ExitOnError)
	email := fs.String("email", "", "Admin email")
	name := fs.String("name", "", "Admin name")
	phone := fs.String("phone", "", "Admin phone number")

	fs.Parse(os.Args[2:])

	// Interactive mode if arguments are missing
	reader := bufio.NewReader(os.Stdin)

	if *email == "" {
		fmt.Print("Enter Admin Email: ")
		val, _ := reader.ReadString('\n')
		*email = strings.TrimSpace(val)
	}

	if *name == "" {
		fmt.Print("Enter Admin Name: ")
		val, _ := reader.ReadString('\n')
		*name = strings.TrimSpace(val)
	}

	if *phone == "" {
		fmt.Print("Enter Admin Phone (e.g. 628123456789): ")
		val, _ := reader.ReadString('\n')
		*phone = strings.TrimSpace(val)
	}

	if *email == "" || *name == "" {
		log.Fatal("Email and Name are required")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := database.InitDB(dsn)
	if err != nil {
		log.Fatalf("Failed to connect DB: %v", err)
	}

	// Check if user already exists
	var user models.User
	if err := db.Where("email = ?", *email).First(&user).Error; err == nil {
		fmt.Printf("User with email %s already exists. Updating to Admin...\n", *email)
		user.UserType = models.UserTypeAdmin
		user.Name = *name
		if *phone != "" {
			user.Phone = *phone
		}
		if err := db.Save(&user).Error; err != nil {
			log.Fatalf("Failed to update user: %v", err)
		}
	} else {
		user = models.User{
			Email:    *email,
			Name:     *name,
			Phone:    *phone,
			UserType: models.UserTypeAdmin,
		}
		if err := db.Create(&user).Error; err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
	}

	fmt.Printf("Successfully registered/updated Admin: %s (%s)\n", user.Name, user.Email)
}
