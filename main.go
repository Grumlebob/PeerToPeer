package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	node "github.com/Grumlebob/PeerToPeer/grpc"
	"google.golang.org/grpc"
)

var lamportTime = int32(0)

type peer struct {
	node.UnimplementedNodeServer
	id             int32
	lamportTime    int32
	responseNeeded int32
	clients        map[int32]node.NodeClient
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
		lamportTime:    lamportTime,
		responseNeeded: 999,
		clients:        make(map[int32]node.NodeClient),
		ctx:            ctx,
		state:          RELEASED,
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", fmt.Sprintf("localhost:%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}
	grpcServer := grpc.NewServer()
	node.RegisterNodeServer(grpcServer, p)

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
		c := node.NewNodeClient(conn)
		p.clients[port] = c
	}

	//We need N-1 responses to enter the critical section
	p.responseNeeded = int32(len(p.clients))

	go func() {
		randomPause(5)
		p.RequestEnterToCriticalSection(p.ctx, &node.Request{Id: p.id})
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		p.sendMessageToAllPeers()
	}
}

func (p *peer) HandlePeerRequest(ctx context.Context, req *node.Request) (*node.Reply, error) {
	//P er den client der svarer på requesten.
	//Req kommer fra anden peer.
	//Reply er det svar peer får.
	p.lamportTime = int32(math.Max(float64(p.lamportTime), float64(req.LamportTime))) + 1
	if p.state == WANTED {
		if req.State == RELEASED {
			p.responseNeeded--
		}
		if req.State == WANTED {
			if req.LamportTime < p.lamportTime {
				p.responseNeeded--
			} else if req.LamportTime == p.lamportTime && req.Id < p.id {
				p.responseNeeded--
			}
		}
	}
	rep := &node.Reply{Id: p.id, State: p.state, LamportTime: p.lamportTime}
	return rep, nil
}

func (p *peer) RequestEnterToCriticalSection(ctx context.Context, req *node.Request) (*node.Reply, error) {
	//P Requests to enter critical section, and sends a request to all other peers.
	//WANTS TO ENTER
	log.Printf("Requesting to enter critical section \n")
	p.lamportTime++
	p.state = WANTED
	p.responseNeeded = int32(len(p.clients))
	p.sendMessageToAllPeers()
	for p.responseNeeded > 0 {

	}
	if p.responseNeeded == 0 {
		p.TheSimulatedCriticalSection()
	}
	//Out of the critical section
	reply := &node.Reply{Id: p.id, State: p.state, LamportTime: p.lamportTime}
	return reply, nil
}

func (p *peer) TheSimulatedCriticalSection() {
	lamportTime++
	p.state = HELD
	log.Printf("%v is in critical section \n", p.id)
	time.Sleep(4 * time.Second)
	//EXITING CRITICAL SECTION
	lamportTime++
	p.responseNeeded = int32(len(p.clients))
	p.state = RELEASED
	log.Printf("%v is out of the critical section \n", p.id)
	p.sendMessageToAllPeers()
}

func (p *peer) sendMessageToAllPeers() {
	lamportTime++
	request := &node.Request{Id: p.id, State: p.state, LamportTime: p.lamportTime}
	for _, client := range p.clients {
		reply, err := client.HandlePeerRequest(p.ctx, request)
		if err != nil {
			log.Println("something went wrong")
		}
		//log.Printf("Reply: ID %v, State: %s, Lamport: %v, Responses needed: %v \n", reply.Id, reply.State, reply.LamportTime, p.responseNeeded)
		log.Printf("timestamp: %v \n", reply.LamportTime)
	}
}

func randomPause(max int) {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(max*1000)))
}
