package goplum

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/csmith/goplum/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"log"
	"net"
)

var (
	apiPort = flag.Int("api-port", 7586, "Port to use for the GoPlum API")
)

type GrpcServer struct {
	plum   *Plum
	server *grpc.Server
}

func NewGrpcServer(plum *Plum) *GrpcServer {
	return &GrpcServer{
		plum: plum,
	}
}

func (s *GrpcServer) Start() {
	certs, pool, err := api.LoadCertificates()
	if err != nil {
		log.Printf("Not starting API: unable to load certificates: %v", err)
		return
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *apiPort))
	if err != nil {
		log.Fatalf("Unable to listen on port %d for API requests: %v", *apiPort, err)
	}

	log.Printf("Starting API server on port %d", *apiPort)
	s.server = grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: certs,
		ClientCAs:    pool,
	})))
	s.server.RegisterService(api.ServiceDesc, s)
	if err := s.server.Serve(lis); err != nil {
		log.Printf("Error serving API: %v", err)
	}
}

func (s *GrpcServer) Stop() {
	if s.server != nil {
		s.server.Stop()
	}
}

func (s *GrpcServer) Results(_ *api.Empty, rs api.GoPlum_ResultsServer) error {
	var (
		l CheckListener
		c = make(chan error, 1)
	)

	l = func(check *ScheduledCheck, result Result) {
		err := rs.Send(&api.Result{
			Check:  check.Name,
			Time:   check.LastRun.Unix(),
			Result: s.convertState(result.State),
			Detail: result.Detail,
		})
		if err != nil {
			s.plum.RemoveCheckListener(l)
			c <- err
		}
	}

	defer close(c)

	s.plum.AddCheckListener(l)

	return <-c
}

func (s *GrpcServer) convertState(state CheckState) api.Status {
	switch state {
	case StateIndeterminate:
		return api.Status_INDETERMINATE
	case StateGood:
		return api.Status_GOOD
	case StateFailing:
		return api.Status_FAILING
	default:
		return api.Status_INDETERMINATE
	}
}
