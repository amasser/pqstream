# pqstream

A library for streaming data from postgres and running functions on real-time notifications.

## Step 1: Create pg_notify triggers

Example notification on a user table that publishes JSON:

```sql
CREATE OR REPLACE FUNCTION notify_user_function()
    RETURNS TRIGGER
    LANGUAGE plpgsql
AS $function$
BEGIN
    IF TG_OP = 'INSERT' THEN
        PERFORM pg_notify('users', format('{ "type": "insert", "table": "users", "id": %s, "auth_id": "%s", "name": "%s", "email": "%s", "phone": "%s" }', NEW.id, NEW.auth_id, NEW.name, NEW.email, NEW.phone));
    ELSIF TG_OP = 'UPDATE' THEN
        PERFORM pg_notify('users', format('{ "type": "update", "table": "users", "id": %s, "auth_id": "%s", "name": "%s", "email": "%s", "phone": "%s" }', NEW.id, NEW.auth_id, NEW.name, NEW.email, NEW.phone));
    ELSIF TG_OP = 'DELETE' THEN
        PERFORM pg_notify('users', format('{ "type": "delete", "table": "users", "id": %s, "auth_id": "%s", "name": "%s", "email": "%s", "phone": "%s" }', OLD.id, OLD.auth_id, OLD.name, OLD.email, OLD.phone));
    END IF;
    RETURN NEW;
END;
$function$;

CREATE TRIGGER nofify_user_trigger
    AFTER INSERT OR UPDATE OR DELETE
          ON users
              FOR EACH ROW
EXECUTE PROCEDURE notify_user_function();
```


## Step 2: Create a pqstream Client

Example: 

```go
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
        //PreHandlers are optional functions to run before Handlers
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
        //Only a single Handler is required to run the client
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
        //PostHandlers are optional functions to run after Handlers
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
        //ErrorHandler processes errors encountered by the client
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
```

## The Handler Interface
The client's PreHandlers, Handlers, and PostHandlers all conform to the following interface signature:
```go
Process(notification *pq.Notification) error

```

You may create types to satisfy this interface, or you can simply pass a first-class function to satisfy the interface by 
calling `HandlerFromHandlerFunc(handler func(notification *pq.Notification) error) Handler` with an anonymous function.
 
Ideas for powerful Handlers for streaming real-time data include:
- POST the notification as a webhook
- Stream the notification to a websocket connection(see https://godoc.org/github.com/gorilla/websocket)
- Publis the notification to a NATS channel
- Publish the notification to a Kafka topic
- Publis the notification to RabbitMQ
- Send an email based on information in the notification
- Send a text based on information in the notification

## GoDoc
--
    import "github.com/autom8ter/pqstream"


## Usage

#### type Client

```go
type Client struct {
}
```

A Client runs Handlers on inbound streams of notifications from postgres LISTEN
NOTIFY

#### func  NewClient

```go
func NewClient(channels []string, config *Config, handlerset *HandlerSet) (*Client, error)
```
NewClient provides a fully configures LISTEN NOTIFY client

#### func (*Client) Start

```go
func (c *Client) Start() error
```
Start starts a LISTEN NOTIFY connection on each channel and runs every
registered handler on each inbound notification

#### type Config

```go
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
```

Config provides a LISTEN NOTIFY client with the necessary configuration values
to create listeners. It is recommended to retrieve these values from
environmental values or remote config.

#### func (*Config) ConnInfo

```go
func (c *Config) ConnInfo() string
```
ConnInfo returns the database connection info

#### type ErrHandlerFunc

```go
type ErrHandlerFunc func(err error)
```

ErrHandlerFunc handles an error in any way

#### type Handler

```go
type Handler interface {
	Process(notification *pq.Notification) error
}
```

A Handler runs a function on a received postgres notification

#### func  HandlerFromHandlerFunc

```go
func HandlerFromHandlerFunc(handler func(notification *pq.Notification) error) Handler
```
HandlerFromHandlerFunc is a helper function to create a Handler

#### type HandlerFunc

```go
type HandlerFunc func(notification *pq.Notification) error
```

A HandlerFunc is a first class function that satisfies the Handler
interface(think http.HandlerFunc)

#### func (HandlerFunc) Process

```go
func (h HandlerFunc) Process(notification *pq.Notification) error
```
Process runs itself on a received postgres notification

#### type HandlerSet

```go
type HandlerSet struct {
	PreHandlers  []Handler
	Handlers     []Handler
	PostHandlers []Handler
	ErrorHandler ErrHandlerFunc
}
```

HandlerSet is a set of interface/first-class functions that run logic on inbound
notifications & errors in real time
