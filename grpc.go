package goplum

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"

	"chameth.com/goplum/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	apiPort   = flag.Int("api-port", 7586, "Port to use for the GoPlum API")
	caCert    = flag.String("ca-cert", "ca.crt", "Path to the certificate of the certificate authority for the API")
	localCert = flag.String("cert", "goplum.crt", "Path to the certificate to use for the API")
	localKey  = flag.String("key", "goplum.key", "Path to the key to use for the API")
)

type GrpcServer struct {
	api.UnimplementedGoPlumServer
	plum   *Plum
	server *grpc.Server
}

func NewGrpcServer(plum *Plum) *GrpcServer {
	return &GrpcServer{
		plum: plum,
	}
}

// Start begins serving the API in the background. If the certificates can't be loaded then goplum runs without an API,
// and no error is returned.
func (s *GrpcServer) Start() error {
	certs, pool, err := api.LoadCertificates(*localCert, *localKey, *caCert)
	if err != nil {
		log.Printf("Not starting API: unable to load certificates: %v", err)
		return nil
	}

	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", fmt.Sprintf("0.0.0.0:%d", *apiPort))
	if err != nil {
		return fmt.Errorf("unable to listen on port %d for API requests: %w", *apiPort, err)
	}

	log.Printf("Starting API server on port %d", *apiPort)
	s.server = grpc.NewServer(
		grpc.Creds(
			credentials.NewTLS(
				&tls.Config{
					ClientAuth:   tls.RequireAndVerifyClientCert,
					Certificates: certs,
					ClientCAs:    pool,
					MinVersion:   tls.VersionTLS13,
				},
			),
		),
	)
	api.RegisterGoPlumServer(s.server, s)
	go func() {
		if err := s.server.Serve(lis); err != nil {
			log.Printf("Error serving API: %v", err)
		}
	}()

	return nil
}

func (s *GrpcServer) Stop() {
	if s.server != nil {
		s.server.Stop()
	}
}

func (s *GrpcServer) Results(_ *api.Empty, rs api.GoPlum_ResultsServer) error {
	// Results are queued so a slow client doesn't stall check runners; if the queue fills up, results are dropped.
	results := make(chan *api.Result, 100)
	l := CheckListener(
		func(check *ScheduledCheck, result Result) {
			select {
			case results <- &api.Result{
				Check:  check.Name,
				Time:   check.LastRun.Unix(),
				Result: s.convertState(result.State),
				Detail: result.Detail,
				Facts:  s.convertFacts(result.Facts),
			}:
			default:
			}
		},
	)

	s.plum.AddCheckListener(l)
	defer s.plum.RemoveCheckListener(l)

	for {
		select {
		case <-rs.Context().Done():
			return rs.Context().Err()
		case r := <-results:
			if err := rs.Send(r); err != nil {
				return err
			}
		}
	}
}

func (s *GrpcServer) GetChecks(_ context.Context, _ *api.Empty) (*api.CheckList, error) {
	checks := make([]*api.Check, 0, len(s.plum.Checks))
	for i := range s.plum.Checks {
		checks = append(checks, s.convertCheck(s.plum.Checks[i]))
	}
	return &api.CheckList{Checks: checks}, nil
}

func (s *GrpcServer) GetCheck(_ context.Context, name *api.CheckName) (*api.Check, error) {
	if name == nil || len(name.GetName()) == 0 {
		return nil, errors.New("no name specified")
	}

	check, ok := s.plum.Checks[name.GetName()]
	if ok {
		return s.convertCheck(check), nil
	}

	return nil, fmt.Errorf("no check found with name: %s", name.GetName())
}

func (s *GrpcServer) SuspendCheck(_ context.Context, name *api.CheckName) (*api.Check, error) {
	if name == nil || len(name.GetName()) == 0 {
		return nil, errors.New("no name specified")
	}

	check := s.plum.Suspend(name.GetName())
	if check == nil {
		return nil, fmt.Errorf("no check found with name: %s", name.GetName())
	}

	return s.convertCheck(check), nil
}

func (s *GrpcServer) ResumeCheck(_ context.Context, name *api.CheckName) (*api.Check, error) {
	if name == nil || len(name.GetName()) == 0 {
		return nil, errors.New("no name specified")
	}

	check := s.plum.Unsuspend(name.GetName())
	if check == nil {
		return nil, fmt.Errorf("no check found with name: %s", name.GetName())
	}

	return s.convertCheck(check), nil
}

func (s *GrpcServer) convertCheck(check *ScheduledCheck) *api.Check {
	return &api.Check{
		Name:      check.Name,
		Type:      check.Type,
		LastRun:   check.LastRun.Unix(),
		Settled:   check.Settled,
		State:     s.convertState(check.State),
		Suspended: check.Suspended,
		History:   s.convertHistory(check),
	}
}

func (s *GrpcServer) convertHistory(check *ScheduledCheck) []*api.Result {
	var res []*api.Result
	for _, r := range check.History {
		if r == nil {
			continue
		}
		res = append(
			res,
			&api.Result{
				Check:  check.Name,
				Time:   r.Time.Unix(),
				Result: s.convertState(r.State),
				Detail: r.Detail,
				Facts:  s.convertFacts(r.Facts),
			},
		)
	}
	return res
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

func (s *GrpcServer) convertFacts(facts map[Fact]any) []*api.Fact {
	res := make([]*api.Fact, 0, len(facts))
	for i := range facts {
		res = append(
			res,
			&api.Fact{
				Name:  string(i),
				Value: s.convertFactValue(facts[i]),
			},
		)
	}
	return res
}

func (s *GrpcServer) convertFactValue(i any) api.FactValue {
	if v, ok := i.(int64); ok {
		return &api.Fact_Int{Int: v}
	}
	if v, ok := i.(string); ok {
		return &api.Fact_Str{Str: v}
	}
	return nil
}
