package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"time"

	"github.com/r-medina/gmaj"
	"github.com/r-medina/gmaj/gmajpb"

	"golang.org/x/net/context"
	"google.golang.org/grpc/grpclog"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var config struct {
	id         string
	addr       string
	parentAddr string
	debug      bool
	pprofAddr  string
}

var (
	app = kingpin.New("gmaj-server", "GMaj server daemon").
		PreAction(startPprof).Action(runServer).DefaultEnvars()

	log grpclog.Logger
)

func init() {
	app.Flag("id", "custom ID to use instead of than hashing address").StringVar(&config.id)
	app.Flag("addr", "address on which to start server").StringVar(&config.addr)
	app.Flag("parent-addr", "address of node to join").StringVar(&config.parentAddr)
	app.Flag("debug", "whether debug mode is on").Default("false").BoolVar(&config.debug)
	app.Flag("pprof-addr", "address for running pprof tools").StringVar(&config.pprofAddr)

	log = gmaj.Log
}

func main() {
	if _, err := app.Parse(os.Args[1:]); err != nil {
		log.Fatalf("command line parsing failed: %v", err)
	}
}

func runServer(_ *kingpin.ParseContext) error {
	var parent *gmajpb.Node
	if config.parentAddr != "" {
		conn, err := gmaj.Dial(config.parentAddr)
		if err != nil {
			log.Fatalf("dialing parent %v failed: %v", config.parentAddr, err)
		}

		client := gmajpb.NewGMajClient(conn)
		id, err := client.GetID(context.Background(), &gmajpb.GetIDRequest{})
		_ = conn.Close()
		if err != nil {
			log.Fatalf("getting parent ID failed: %v", err)
		}

		parent = &gmajpb.Node{Id: id.Id, Addr: config.parentAddr}
		log.Printf("attaching to %v", gmaj.IDToString(parent.Id))
	}

	var opts []gmaj.NodeOption

	opts = append(opts, gmaj.WithAddress(config.addr))

	if config.id != "" {
		id, err := gmaj.NewID(config.id)
		if err != nil {
			log.Fatalf("parsing ID failed: %v", err)
		}
		opts = append(opts, gmaj.WithID(id))
	}

	node, err := gmaj.NewNode(parent, opts...)
	if err != nil {
		log.Fatalf("faild to instantiate node: %v", err)
	}

	log.Printf("%+v", node)

	if config.debug {
		go func() {
			for range time.Tick(5 * time.Second) {
				log.Println(node)
				log.Println(node.DatastoreString())
			}
		}()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	sig := <-stop
	log.Printf("received signal %v", sig)

	log.Println("shutting down")
	node.Shutdown()

	return nil
}

func startPprof(_ *kingpin.ParseContext) error {
	if config.pprofAddr == "" {
		return nil
	}

	log.Printf("running pprof server on %s", config.pprofAddr)
	go func() {
		log.Println(http.ListenAndServe(config.pprofAddr, nil))
	}()

	return nil
}
