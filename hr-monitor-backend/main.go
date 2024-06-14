package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/confluentinc/confluent-kafka-go/kafka"
    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

var clients = make(map[*websocket.Conn]bool) // connected clients
var broadcast = make(chan []byte)            // broadcast channel

func main() {
    http.HandleFunc("/ws", handleConnections)

    go handleMessages()
    go consumeKafkaMessages()

    log.Println("http server started on :8000")
    err := http.ListenAndServe(":8000", nil)
    if err != nil {
        log.Fatal("ListenAndServe: ", err)
    }
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer ws.Close()

    clients[ws] = true

    for {
        _, msg, err := ws.ReadMessage()
        if err != nil {
            delete(clients, ws)
            break
        }
        broadcast <- msg
    }
}

func handleMessages() {
    for {
        msg := <-broadcast
        for client := range clients {
            err := client.WriteMessage(websocket.TextMessage, msg)
            if err != nil {
                log.Printf("error: %v", err)
                client.Close()
                delete(clients, client)
            }
        }
    }
}

func consumeKafkaMessages() {
    kafkaBroker := os.Getenv("KAFKA_BROKER")
    if kafkaBroker == "" {
        log.Fatal("KAFKA_BROKER environment variable must be set")
    }

    var c *kafka.Consumer
    var err error
    for {
        c, err = kafka.NewConsumer(&kafka.ConfigMap{
            "bootstrap.servers": kafkaBroker,
            "group.id":          "hr-monitor-group",
            "auto.offset.reset": "earliest",
        })
        if err == nil {
            break
        }
        log.Printf("Failed to connect to Kafka: %v. Retrying in 5 seconds...", err)
        time.Sleep(5 * time.Second)
    }

    defer c.Close()

    c.SubscribeTopics([]string{"sensor_data"}, nil)

    sigchan := make(chan os.Signal, 1)
    signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

    run := true

    for run {
        select {
        case sig := <-sigchan:
            fmt.Printf("Caught signal %v: terminating\n", sig)
            run = false
        default:
            ev := c.Poll(100)
            switch e := ev.(type) {
            case *kafka.Message:
                fmt.Printf("Message on %s: %s\n", e.TopicPartition, string(e.Value))
                broadcast <- e.Value
            case kafka.Error:
                fmt.Fprintf(os.Stderr, "%% Error: %v\n", e)
                run = false
            default:
                // Ignore other event types
            }
        }
    }
}
