package main

import (
	"context"

	"google.golang.org/grpc"

	mts "github.com/openmts/mts"
)

func withGRPCOperationAudit(op operation, handler grpc.MethodHandler) grpc.MethodHandler {
	action, ok := grpcAuditAction(op.Name)
	if !ok {
		return handler
	}
	return func(
		service any,
		ctx context.Context,
		decode func(any) error,
		interceptor grpc.UnaryServerInterceptor,
	) (any, error) {
		runtime := service.(*grpcService).runtime
		auditInterceptor := func(
			ctx context.Context,
			req any,
			_ *grpc.UnaryServerInfo,
			next grpc.UnaryHandler,
		) (any, error) {
			event := grpcAuditEvent(runtime, ctx, action, req)
			response, err := next(ctx, req)
			if err == nil {
				runtime.audit.record(event)
			}
			return response, err
		}
		return handler(service, ctx, decode, chainGRPCUnaryInterceptors(interceptor, auditInterceptor))
	}
}

func grpcAuditAction(name string) (string, bool) {
	switch name {
	case "login", "logout", "change_password",
		"create_user", "update_user", "delete_user",
		"grant_database_permission", "revoke_database_permission",
		"create_database", "drop_database", "create_retention_policy",
		"reload_config", "flush", "compact", "apply_retention",
		"storage_snapshot", "delete_storage_snapshot",
		"enable_downsample_policy", "disable_downsample_policy",
		"drop_downsample_policy", "reset_downsample_policy",
		"run_downsample_policy":
		return name, true
	case "set_user_password":
		return "set_password", true
	case "create_downsample_policy":
		return "upsert_downsample_policy", true
	default:
		return "", false
	}
}

func grpcAuditEvent(
	runtime *serverRuntime,
	ctx context.Context,
	action string,
	req any,
) auditEvent {
	event := auditEvent{UserName: runtime.grpcActor(ctx), Action: action}
	switch request := req.(type) {
	case *loginRequest:
		event.UserName = request.UserName
	case *createUserRequest:
		event.UserName = request.Name
	case *mts.User:
		event.UserName = request.Name
	case *userNameRequest:
		event.UserName = request.Name
	case *setUserPasswordRequest:
		event.UserName = request.UserName
	case *databasePermissionRequest:
		event.UserName = request.UserName
		event.Database = request.Database
		event.Detail = string(request.Permission)
	case *databaseRequest:
		event.Database = request.Name
	case *grpcRetentionPolicyRequest:
		event.Database = request.Database
		event.Detail = request.Policy.Name
	case *mts.DownsamplePolicy:
		event.Detail = request.Name
	case *downsamplePolicyRequest:
		event.Detail = request.Name
	case *downsamplePolicyRangeRequest:
		event.Detail = request.Name
	case *storageSnapshotDeleteRequest:
		event.Detail = request.Name
	}
	return event
}
