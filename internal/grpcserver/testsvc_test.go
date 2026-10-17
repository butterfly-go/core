package grpcserver

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"

	corelog "butterfly.orx.me/core/log"
)

const svcName = "test.Svc"

// registerTestService registers a hand-rolled service (no generated code) with
// unary methods that return each status code, plus panicking and streaming
// methods.
func registerTestService(s *grpc.Server) {
	failWith := func(c codes.Code) func(context.Context) (any, error) {
		return func(context.Context) (any, error) { return nil, status.Error(c, "failed: "+c.String()) }
	}
	unary := map[string]func(context.Context) (any, error){
		"Echo": func(ctx context.Context) (any, error) {
			corelog.FromContext(ctx).Info("in handler")
			return wrapperspb.String("ok"), nil
		},
		"NotFound":         failWith(codes.NotFound),
		"InvalidArgument":  failWith(codes.InvalidArgument),
		"Unavailable":      failWith(codes.Unavailable),
		"DeadlineExceeded": failWith(codes.DeadlineExceeded),
		"Internal":         failWith(codes.Internal),
		"Unknown":          failWith(codes.Unknown),
		"Panic":            func(context.Context) (any, error) { panic("kaboom") },
		// Reaches the handler only if the user interceptor does not panic first.
		"UserPanic": func(context.Context) (any, error) { return wrapperspb.String("ok"), nil },
	}

	desc := &grpc.ServiceDesc{
		ServiceName: svcName,
		HandlerType: (*any)(nil),
		Streams: []grpc.StreamDesc{
			{
				StreamName:    "StreamOK",
				ServerStreams: true,
				Handler: func(_ any, stream grpc.ServerStream) error {
					if err := stream.RecvMsg(new(wrapperspb.StringValue)); err != nil {
						return err
					}
					corelog.FromContext(stream.Context()).Info("in stream handler")
					return stream.SendMsg(wrapperspb.String("x"))
				},
			},
			{
				StreamName:    "StreamPanic",
				ServerStreams: true,
				Handler: func(_ any, stream grpc.ServerStream) error {
					if err := stream.RecvMsg(new(wrapperspb.StringValue)); err != nil {
						return err
					}
					panic("stream kaboom")
				},
			},
		},
	}
	for name, fn := range unary {
		desc.Methods = append(desc.Methods, grpc.MethodDesc{MethodName: name, Handler: unaryHandler(name, fn)})
	}
	s.RegisterService(desc, struct{}{})
}

func unaryHandler(name string, fn func(context.Context) (any, error)) func(any, context.Context, func(any) error, grpc.UnaryServerInterceptor) (any, error) {
	return func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		if err := dec(new(wrapperspb.StringValue)); err != nil {
			return nil, err
		}
		call := func(ctx context.Context, _ any) (any, error) { return fn(ctx) }
		if interceptor == nil {
			return call(ctx, nil)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + svcName + "/" + name}
		return interceptor(ctx, nil, info, call)
	}
}
