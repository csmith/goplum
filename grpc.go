package goplum

import (
	"context"
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
	caCert    = flag.String("ca-cert", "ca.crt", "Path to the certificate of the certificate authority for the API")
	localCert = flag.String("cert", "goplum.crt", "Path to the certificate to use for the API")
	localKey  = flag.String("key", "goplum.key", "Path to the key to use for the API")
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
	certs, pool, err := api.LoadCertificates(*localCert, *localKey, *caCert)
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

func (s *GrpcServer) GetChecks(_ context.Context, _ *api.Empty) (*api.CheckList, error) {
	var checks []*api.Check
	for i := range s.plum.Checks {
		checks = append(checks, s.convertCheck(s.plum.Checks[i]))
	}
	return &api.CheckList{Checks: checks}, nil
}

func (s *GrpcServer) GetCheck(_ context.Context, name *api.CheckName) (*api.Check, error) {
	if name == nil || len(name.Name) == 0 {
		return nil, fmt.Errorf("no name specified")
	}

	check, ok := s.plum.Checks[name.Name]
	if ok {
		return s.convertCheck(check), nil
	}

	return nil, fmt.Errorf("no check found with name: %s", name.Name)
}

func (s *GrpcServer) SuspendCheck(_ context.Context, name *api.CheckName) (*api.Check, error) {
	if name == nil || len(name.Name) == 0 {
		return nil, fmt.Errorf("no name specified")
	}

	check := s.plum.Suspend(name.Name)
	if check == nil {
		return nil, fmt.Errorf("no check found with name: %s", name.Name)
	} else {
		return s.convertCheck(check), nil
	}
}

func (s *GrpcServer) ResumeCheck(_ context.Context, name *api.CheckName) (*api.Check, error) {
	if name == nil || len(name.Name) == 0 {
		return nil, fmt.Errorf("no name specified")
	}

	check := s.plum.Unsuspend(name.Name)
	if check == nil {
		return nil, fmt.Errorf("no check found with name: %s", name.Name)
	} else {
		return s.convertCheck(check), nil
	}
}

func (s *GrpcServer) convertCheck(check *ScheduledCheck) *api.Check {
	return &api.Check{
		Name:      check.Name,
		Type:      check.Type,
		LastRun:   check.LastRun.Unix(),
		Settled:   check.Settled,
		State:     s.convertState(check.State),
		Suspended: check.Suspended,
	}
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
