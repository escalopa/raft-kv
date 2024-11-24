// Code generated by protoc-gen-catalystgo. DO NOT EDIT.

package raft

import (
	context "context"
	_ "embed"

	go_grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

//go:embed raft.swagger.json
var swaggerJSON []byte

type RaftServiceServiceDesc struct {
	svc RaftServiceServer
	i   grpc.UnaryServerInterceptor
}

func NewRaftServiceServiceDesc(svc RaftServiceServer) *RaftServiceServiceDesc {
	return &RaftServiceServiceDesc{
		svc: svc,
	}
}

func (d *RaftServiceServiceDesc) RegisterGRPC(s *grpc.Server) {
	RegisterRaftServiceServer(s, d.svc)
}

func (d *RaftServiceServiceDesc) RegisterHTTP(ctx context.Context, mux *runtime.ServeMux) error {
	if d.i == nil {
		return RegisterRaftServiceHandlerServer(ctx, mux, d.svc)
	}

	return RegisterRaftServiceHandlerServer(ctx, mux, &proxyRaftServiceServer{
		RaftServiceServer: d.svc,
		interceptor:       d.i,
	})
}

func (d *RaftServiceServiceDesc) SwaggerJSON() []byte {
	return swaggerJSON
}

// WithHTTPUnaryInterceptor adds GRPC Server interceptors for the HTTP unary endpoints. Call again to add more.
func (d *RaftServiceServiceDesc) WithHTTPUnaryInterceptor(i grpc.UnaryServerInterceptor) {
	if d.i == nil {
		d.i = i
	} else {
		d.i = go_grpc_middleware.ChainUnaryServer(d.i, i)
	}
}

type proxyRaftServiceServer struct {
	RaftServiceServer
	interceptor grpc.UnaryServerInterceptor
}

func (p *proxyRaftServiceServer) AppendEntry(ctx context.Context, req *AppendEntryRequest) (*AppendEntryResponse, error) {
	info := &grpc.UnaryServerInfo{
		Server:     p.RaftServiceServer,
		FullMethod: "/raft.RaftService/AppendEntry",
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return p.RaftServiceServer.AppendEntry(ctx, req.(*AppendEntryRequest))
	}

	resp, err := p.interceptor(ctx, req, info, handler)
	if err != nil || resp == nil {
		return nil, err
	}

	return resp.(*AppendEntryResponse), nil
}

func (p *proxyRaftServiceServer) RequestVote(ctx context.Context, req *RequestVoteRequest) (*RequestVoteResponse, error) {
	info := &grpc.UnaryServerInfo{
		Server:     p.RaftServiceServer,
		FullMethod: "/raft.RaftService/RequestVote",
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return p.RaftServiceServer.RequestVote(ctx, req.(*RequestVoteRequest))
	}

	resp, err := p.interceptor(ctx, req, info, handler)
	if err != nil || resp == nil {
		return nil, err
	}

	return resp.(*RequestVoteResponse), nil
}
