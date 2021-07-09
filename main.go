package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/migration-agent/pkg/migrate"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	Version   = "v0.0.0-dev"
	GitCommit = "HEAD"
	config    migrate.MigrationConfig
)

func main() {
	app := cli.NewApp()
	app.Name = "migration-agent"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Usage = "Agent migrates rke files and data node to rke2"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "kubeconfig",
			EnvVar:      "KUBECONFIG",
			Destination: &config.KubeConfig,
		},
		cli.StringFlag{
			Name:        "data-dir",
			EnvVar:      "DATADIR",
			Destination: &config.DataDir,
			Value:       "/var/lib/rancher/rke2",
		},
		cli.StringFlag{
			Name:        "snapshot",
			EnvVar:      "SNAPSHOT",
			Destination: &config.Snapshot,
		},
		&cli.StringFlag{
			Name:        "s3-endpoint",
			Usage:       "S3 endpoint url",
			Destination: &config.EtcdS3Endpoint,
			Value:       "s3.amazonaws.com",
		},
		&cli.StringFlag{
			Name:        "s3-endpoint-ca",
			Usage:       "S3 custom CA cert to connect to S3 endpoint",
			Destination: &config.EtcdS3EndpointCA,
		},
		&cli.BoolFlag{
			Name:        "s3-skip-ssl-verify",
			Usage:       "Disables S3 SSL certificate validation",
			Destination: &config.EtcdS3SkipSSLVerify,
		},
		&cli.StringFlag{
			Name:        "s3-access-key",
			Usage:       "S3 access key",
			EnvVar:      "AWS_ACCESS_KEY_ID",
			Destination: &config.EtcdS3AccessKey,
		},
		&cli.StringFlag{
			Name:        "s3-secret-key",
			Usage:       "S3 secret key",
			EnvVar:      "AWS_SECRET_ACCESS_KEY",
			Destination: &config.EtcdS3SecretKey,
		},
		&cli.StringFlag{
			Name:        "s3-bucket",
			Usage:       "S3 bucket name",
			Destination: &config.EtcdS3BucketName,
		},
		&cli.StringFlag{
			Name:        "s3-region",
			Usage:       "S3 region / bucket location (optional)",
			Destination: &config.EtcdS3Region,
			Value:       "us-east-1",
		},
		&cli.StringFlag{
			Name:        "s3-folder",
			Usage:       "S3 folder",
			Destination: &config.EtcdS3Folder,
		},
		&cli.StringFlag{
			Name:        "node-name",
			Usage:       "Node Name",
			Destination: &config.NodeName,
		},
		&cli.BoolFlag{
			Name:        "disable-etcd-restore",
			Usage:       "Disable etcd restoration on the migrated node",
			Destination: &config.DisableETCDRestore,
		},
		&cli.StringSliceFlag{
			Name:  "registry",
			Usage: "Configure private registry TLS paths, syntax should be <registry url>,<ca cert path>,<cert path>,<key path>",
			Value: &config.RegistriesTLS,
		},
	}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) {
	logrus.Info("Starting agent")
	ctx := signals.SetupSignalHandler(context.Background())

	kubeConfig, err := kubeconfig.GetNonInteractiveClientConfig(config.KubeConfig).ClientConfig()
	if err != nil {
		logrus.Fatalf("failed to find kubeconfig: %v", err)
	}

	var k8sConn bool
	sc, err := migrate.NewContext(ctx, kubeConfig)
	if err != nil {
		if config.NodeName == "" {
			logrus.Fatalf("failed to find establish kubernetes connection and node-name is empty: %v", err)
		}
		logrus.Warnf("failed to establish kubernetes connection, will use node-name statically")
	} else {
		k8sConn = true
		if err := sc.Start(ctx); err != nil {
			logrus.Fatalf("failed to start factories: %v", err)
		}
	}

	agent, err := migrate.New(ctx, sc, &config, k8sConn)
	if err != nil {
		logrus.Fatalf("failed to create a migration agent on node: %v", err)
	}

	if err := agent.Do(ctx); err != nil {
		logrus.Fatalf("failed to run migrate on node: %v", err)
	}

	logrus.Infof("Node has been migrated successfully")
}
