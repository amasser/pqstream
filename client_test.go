package pqstream_test

import (
	"encoding/json"
	"github.com/autom8ter/pqstream"
	"github.com/lib/pq"
	"log"
	"testing"
)

func TestFull(t *testing.T) {
	//create a config for db listeners
	config := &pqstream.Config{
		Host:     "localhost",
		Port:     "5432",
		User:     "postgres",
		Password: "postgres",
		Database: "postgres",
		Verbose:  true,
	}
	//create a handlerset for handling database notifications
	handlerSet := &pqstream.HandlerSet{
		PreHandlers: []pqstream.Handler{
			pqstream.HandlerFromHandlerFunc(func(notification *pq.Notification) error {
				log.Println("inside pre handler 1")
				return nil
			}),
			pqstream.HandlerFromHandlerFunc(func(notification *pq.Notification) error {
				log.Println("inside pre handler 2")
				return nil
			}),
		},
		Handlers: []pqstream.Handler{
			pqstream.HandlerFromHandlerFunc(func(notification *pq.Notification) error {
				bits, err := json.MarshalIndent(notification, "", "    ")
				if err != nil {
					return err
				}
				log.Printf("notification received: %s", string(bits))
				return nil
			}),
		},
		PostHandlers: []pqstream.Handler{
			pqstream.HandlerFromHandlerFunc(func(notification *pq.Notification) error {
				log.Println("inside post handler 1")
				return nil
			}),
			pqstream.HandlerFromHandlerFunc(func(notification *pq.Notification) error {
				log.Println("inside post handler 2")
				return nil
			}),
		},
		ErrorHandler: func(err error) {
			log.Println("TEST ERROR: ", err.Error())
		},
	}
	//create a client for running the handlers on each stream
	client, err := pqstream.NewClient([]string{"users"}, config, handlerSet)
	if err != nil {
		log.Fatal(err.Error())
	}
	if err := client.Start(); err != nil {
		log.Fatal(err.Error())
	}
}
