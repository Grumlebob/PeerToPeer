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
		randomPause(10)
		p.RequestEnterToCriticalSection(p.ctx, &node.Request{Id: p.id})
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		p.sendMessageToAllPeers()
	}
}

func (p *peer) HandlePeerRequest(ctx context.Context, req *node.Request) (*node.Reply, error) {
	//Dette er her "serveren/andre nodes" der svarer på en request og sender deres reply.
	//P er den node der SENDER requesten
	//Metoden er den peer der skal sende reply.
	//log.Printf("Got ping from %v, with %s \n", p.id, p.state)
	rep := &node.Reply{Id: p.id}
	return rep, nil
}

func (p *peer) RequestEnterToCriticalSection(ctx context.Context, req *node.Request) (*node.Reply, error) {
	//P Requests to enter critical section, and sends a request to all other peers.
	//WANTS TO ENTER
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
	p.state = HELD
	time.Sleep(5 * time.Second)
	log.Printf("%v is in critical section \n", p.id)
	time.Sleep(5 * time.Second)
	//EXITING CRITICAL SECTION
	p.responseNeeded = int32(len(p.clients))
	p.state = RELEASED
	log.Printf("%v is out of the critical section \n", p.id)
	p.sendMessageToAllPeers()

}

func (p *peer) sendMessageToAllPeers() {
	request := &node.Request{Id: p.id, State: p.state, LamportTime: p.lamportTime}
	for _, client := range p.clients {
		reply, err := client.HandlePeerRequest(p.ctx, request)
		if err != nil {
			log.Println("something went wrong")
		}
		if p.id != reply.Id {
			if reply.LamportTime > lamportTime {
				lamportTime = reply.LamportTime + 1
			} else {
				lamportTime++
			}
		}
		if p.state == WANTED {
			if reply.LamportTime < p.lamportTime {
				p.responseNeeded--
				//log.Printf("response needed: %v \n", p.responseNeeded)
			}
			if reply.LamportTime == p.lamportTime && reply.Id < p.id {
				p.responseNeeded--
				//log.Printf("response needed: %v \n", p.responseNeeded)
			}
		} else if p.state == RELEASED {
			//logic mangler
		}
		//log.Printf("Got reply from id %v: should be same: %v \n ", id, reply.Id)
	}
}

func randomPause(max int) {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(max*1000)))
}
