package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	ping "github.com/Grumlebob/PeerToPeer/grpc"
	"google.golang.org/grpc"
)

type peer struct {
	ping.UnimplementedNodeServer
	id             int32
	lamportTime    int32
	responseNeeded int32
	amountOfPings  map[int32]int32
	clients        map[int32]ping.NodeClient
	ctx            context.Context
	state          string
}

const (
	RELEASED = "Released"
	WANTED   = "Wanted"
	HELD     = "Held"
)

func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + 5000

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &peer{
		id:             ownPort,
		lamportTime:    0,
		responseNeeded: 999,
		amountOfPings:  make(map[int32]int32),
		clients:        make(map[int32]ping.NodeClient),
		ctx:            ctx,
		state:          RELEASED,
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", fmt.Sprintf("localhost:%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}
	grpcServer := grpc.NewServer()
	ping.RegisterNodeServer(grpcServer, p)

	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to server %v", err)
		}
	}()

	for i := 0; i < 3; i++ {
		port := int32(5000) + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		fmt.Printf("Trying to dial: %v\n", port)
		conn, err := grpc.Dial(fmt.Sprintf("localhost:%v", port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Could not connect: %s", err)
		}
		defer conn.Close()
		c := ping.NewNodeClient(conn)
		p.clients[port] = c
	}

	//We need N-1 responses to enter the critical section
	p.responseNeeded = int32(len(p.clients))

	go func() {
		randomPause(10)
		p.CriticalSection(p.ctx, &ping.Request{Id: p.id})
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		p.sendPingToAll()
	}
}

func (p *peer) Broadcast(ctx context.Context, req *ping.Request) (*ping.Reply, error) {
	id := req.Id
	p.amountOfPings[id] += 1
	log.Printf("Got ping from %v, with %s \n", id, p.state)

	rep := &ping.Reply{Id: p.amountOfPings[id]}
	return rep, nil
}

func (p *peer) CriticalSection(ctx context.Context, req *ping.Request) (*ping.Reply, error) {
	//WANTS TO ENTER
	p.state = WANTED
	requestsNeeded := len(p.clients)

	if requestsNeeded == 0 {
		p.TheSimulatedCriticalSection()
	}

	//CRITICAL RESOURCE:
	time.Sleep(5 * time.Second)
	log.Printf("%v is in critical section \n", req.Id)
	time.Sleep(5 * time.Second)

	//EXITING CRITICAL SECTION
	p.state = RELEASED
	p.Broadcast(p.ctx, &ping.Request{Id: p.id})

	rep := &ping.Reply{Id: p.amountOfPings[req.Id]}
	return rep, nil
}

func (p *peer) TheSimulatedCriticalSection() {
	p.state = HELD
	time.Sleep(5 * time.Second)
	log.Printf("%v is in critical section \n", p.id)
	time.Sleep(5 * time.Second)
	p.responseNeeded = int32(len(p.clients))
}

func (p *peer) sendPingToAll() {
	request := &ping.Request{Id: p.id}
	for id, client := range p.clients {
		reply, err := client.Broadcast(p.ctx, request)
		if err != nil {
			fmt.Println("something went wrong")
		}
		if p.id != reply.Id {
			if reply.lamportTime > lamportTime {
				lamportTime = message.LamportTime + 1
			} else {
				lamportTime++
			}
		}
		log.Printf("Got reply from id %v: %v\n", id, reply.Amount)
	}
}

func randomPause(max int) {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(max*40)))
}
