package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"

	tenantpb "github.com/bachelor-thesis-hown3d/chat-api-server/proto/tenant/v1"
	"github.com/bachelor-thesis-hown3d/chat-tenant-server/api"
	"github.com/bachelor-thesis-hown3d/chat-tenant-server/service"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"google.golang.org/grpc"

	"github.com/bachelor-thesis-hown3d/chat-api-server/pkg/health"
	"github.com/bachelor-thesis-hown3d/chat-tenant-server/pkg/k8sutil"
	"github.com/bachelor-thesis-hown3d/chat-tenant-server/pkg/oauth"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var (
	port           = flag.Int("port", 10000, "The server port")
	devel          = flag.Bool("devel", false, "Set the server to development mode (nice log, grpcui etc.)")
	oauthClientID  = flag.String("oauth-client-id", "kubernetes", "oauth Client ID of the issuer")
	oauthIssuerUrl = flag.String("oauth-issuer-url", "https://keycloak:8443/auth/realms/kubernetes", "oauth Client ID of the issuer")
	logger         *zap.Logger
)

func main() {
	k8sutil.CreateKubeconfigFlag()
	flag.Parse()

	if *devel {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}

	//setup oauth issuer
	ctx := context.Background()
	// since we use a bad ssl certificate on localhost, embed a Insecure HTTP Client for oauth to use
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, http.DefaultClient)
	// parse the redirect URL for the port number
	issuerURL, err := url.Parse(*oauthIssuerUrl)
	if err != nil {
		logger.Fatal(err.Error())
	}
	oauthConfig, err := oauth.NewConfig(ctx, issuerURL, *oauthClientID)
	if err != nil {
		logger.Fatal(err.Error())
	}

	grpcServer := grpc.NewServer(
		grpc_middleware.WithUnaryServerChain(
			grpc_zap.UnaryServerInterceptor(logger),
			grpc_auth.UnaryServerInterceptor(oauthConfig.Middleware),
		),
		grpc_middleware.WithStreamServerChain(
			grpc_auth.StreamServerInterceptor(oauthConfig.Middleware),
			grpc_zap.StreamServerInterceptor(logger),
		),
	)

	defer logger.Sync() // flushes buffer, if any

	kubeclient, err := k8sutil.NewClientsetFromKubeconfig()
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to get kubernetes client from config: %v", err))
	}

	certmanagerClient, err := k8sutil.NewCertManagerClientsetFromKubeconfig()
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to get certmanager kube client from config: %v", err))
	}

	healthService := health.NewHealthChecker(kubeclient)

	// tenant proto Service
	tenantService := service.NewTenantServiceImpl(kubeclient, certmanagerClient)
	tenantAPI := api.NewAPIServer(tenantService)
	tenantpb.RegisterTenantServiceServer(grpcServer, tenantAPI)

	grpc_health_v1.RegisterHealthServer(grpcServer, healthService)

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%v", *port))
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to listen on port %v: %v", port, err))
	}

	logger.Info(fmt.Sprintf("Starting grpc server on %v ...", lis.Addr().String()))
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal(fmt.Sprintf("Failed to start grpc Server %v", err))
	}
}
