package services

import (
	"fmt"

	"github.com/philippgille/chromem-go"
)

var db *chromem.DB

func SetupDb() *chromem.DB {
	fmt.Println("Setting up database")
	db = chromem.NewDB()
	return db
}

func CreateTestVectors() {
	fmt.Println("Creating test vectors")
}
