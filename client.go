//go:generate godocdown -o README.md

package pqstream

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"log"
	"sync"
	"time"
)

const pkg = "PQSTREAM"

//A Handler runs a function on a received postgres notification
type Handler interface {
	Process(notification *pq.Notification) error
}

//ErrHandlerFunc handles an error in any way
type ErrHandlerFunc func(err error)

//A HandlerFunc is a first class function that satisfies the Handler interface(think http.HandlerFunc)
type HandlerFunc func(notification *pq.Notification) error

//Process runs itself on a received postgres notification
func (h HandlerFunc) Process(notification *pq.Notification) error {
	return h(notification)
}

//HandlerFromHandlerFunc is a helper function to create a Handler
func HandlerFromHandlerFunc(handler func(notification *pq.Notification) error) Handler {
	return HandlerFunc(handler)
}

//Config provides a LISTEN NOTIFY client with the necessary configuration values to create listeners. It is recommended to retrieve these values from environmental values or remote config.
type Config struct {
	Host         string
	Port         string
	User         string
	Password     string
	Database     string
	SSLMode      string
	SSLCert      string
	SSLKey       string
	SSLRootCert  string
	MaxOpenConns int
	MaxIdleConns int
	Verbose      bool
}

//HandlerSet is a set of interface/first-class functions that run logic on inbound notifications & errors in real time
type HandlerSet struct {
	PreHandlers  []Handler
	Handlers     []Handler
	PostHandlers []Handler
	ErrorHandler ErrHandlerFunc
}

//A Client runs Handlers on inbound streams of notifications from postgres LISTEN NOTIFY
type Client struct {
	channels  []string
	config    *Config
	handlers  *HandlerSet
	listeners map[string]*pq.Listener
}

//NewClient provides a fully configures LISTEN NOTIFY client
func NewClient(channels []string, config *Config, handlerset *HandlerSet) (*Client, error) {
	if handlerset == nil {
		return nil, errors.New("empty handlerset")
	}
	if config == nil {
		return nil, errors.New("empty config")
	}
	if handlerset.ErrorHandler == nil {
		handlerset.ErrorHandler = func(err error) {
			log.Printf("[%s] error: %s", pkg, err.Error())
		}
	}
	if len(handlerset.Handlers) == 0 {
		return nil, fmt.Errorf("[%s] error: %s", pkg, "zero handlers in config")
	}
	if config.Port == "" {
		config.Port = "5432"
	}
	if config.Host == "" {
		config.Host = "localhost"
	}
	if config.User == "" {
		config.User = "postgres"
	}
	if config.Database == "" {
		config.Database = "postgres"
	}
	return &Client{
		channels:  channels,
		config:    config,
		handlers:  handlerset,
		listeners: map[string]*pq.Listener{},
	}, nil
}

//ConnInfo returns the database connection info
func (c *Config) ConnInfo() string {
	if c.SSLCert == "" || c.SSLKey == "" {
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			c.Host, c.Port, c.User, c.Password, c.Database)
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s sslrootcert=%s sslcert=%s sslkey=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode, c.SSLRootCert, c.SSLCert, c.SSLKey)
}

//Start starts a LISTEN NOTIFY connection on each channel and runs every registered handler on each inbound notification
func (c *Client) Start() error {
	return c.start()
}

func (c *Client) start() error {
	db, err := sql.Open("postgres", c.config.ConnInfo())
	if err != nil {
		return fmt.Errorf("failed to open with connection info! %s", err.Error())
	}
	defer db.Close()
	if c.config.MaxOpenConns != 0 {
		db.SetMaxOpenConns(c.config.MaxOpenConns)
	}
	if c.config.MaxIdleConns != 0 {
		db.SetMaxIdleConns(c.config.MaxIdleConns)
	}
	group := sync.WaitGroup{}
	for _, channel := range c.channels {
		group.Add(1)
		go func(ch string) {
			defer group.Done()
			c.listeners[ch] = pq.NewListener(c.config.ConnInfo(), 10*time.Second, 3*time.Minute, func(event pq.ListenerEventType, err error) {
				if err != nil {
					c.handlers.ErrorHandler(fmt.Errorf("event type: %d error: %s\n", event, err.Error()))
					return
				}
			})
			if err := c.listeners[ch].Listen(ch); err != nil {
				c.handlers.ErrorHandler(fmt.Errorf("failed to listen on channel : %s!", ch))
				return
			}
			defer func() {
				if err := c.listeners[ch].Close(); err != nil {
					if c.config.Verbose {
						c.handlers.ErrorHandler(fmt.Errorf("failed to close channel : %s!", ch))
					}
				}
			}()
			for {
				select {
				case n := <-c.listeners[ch].Notify:
					if c.config.Verbose {
						log.Printf("%s received notification %d on channel: %s", pkg, n.BePid, n.Channel)
					}
					if len(c.handlers.PreHandlers) > 0 {
						preWg := sync.WaitGroup{}
						for _, handler := range c.handlers.Handlers {
							preWg.Add(1)
							go func(notification *pq.Notification, h Handler) {
								defer preWg.Done()
								if err := h.Process(notification); err != nil {
									c.handlers.ErrorHandler(fmt.Errorf("failed to pre-process notification! pid: %d, channel: %s error: %s", notification.BePid, notification.Channel, err.Error()))
								}
							}(n, handler)
						}
						preWg.Wait()
					}
					mainWg := sync.WaitGroup{}
					for _, handler := range c.handlers.Handlers {
						mainWg.Add(1)
						go func(notification *pq.Notification, h Handler) {
							defer mainWg.Done()
							if err := h.Process(notification); err != nil {
								c.handlers.ErrorHandler(fmt.Errorf("failed to process notification! pid: %d, channel: %s error: %s", notification.BePid, notification.Channel, err.Error()))
							}
						}(n, handler)
					}
					mainWg.Wait()
					if len(c.handlers.PostHandlers) > 0 {
						postWg := sync.WaitGroup{}
						for _, handler := range c.handlers.PostHandlers {
							postWg.Add(1)
							go func(notification *pq.Notification, h Handler) {
								defer postWg.Done()
								if err := h.Process(notification); err != nil {
									c.handlers.ErrorHandler(fmt.Errorf("failed to post-process notification! pid: %d, channel: %s error: %s", notification.BePid, notification.Channel, err.Error()))
								}
							}(n, handler)
						}
						postWg.Wait()
					}

				case <-time.After(90 * time.Second):
					if c.config.Verbose {
						log.Printf("%s Received no events for 90 seconds, checking connection!", pkg)
					}
					if err := c.listeners[ch].Ping(); err != nil {
						c.handlers.ErrorHandler(fmt.Errorf("failed to ping database for channel: %s error: %s", ch, err.Error()))
					}
					if c.config.Verbose {
						log.Printf("%s Successful database ping!", pkg)
					}
				}
			}
		}(channel)
	}
	group.Wait()
	return nil
}
