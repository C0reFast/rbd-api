package cmd

import (
	"net/http"

	"ichenfu.com/rbd-api/pkg/router"

	"github.com/ceph/go-ceph/rados"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		conn, err := rados.NewConn()
		if err != nil {
			log.Fatal(err)
		}
		if err := conn.ReadDefaultConfigFile(); err != nil {
			log.Fatal(err)
		}
		if err := conn.Connect(); err != nil {
			log.Fatal(err)
		}
		defer conn.Shutdown()
		if err := router.AddWebServices(conn); err != nil {
			log.Fatal(err)
		}

		log.Info("Listening")
		log.Fatal(http.ListenAndServe(":80", nil))
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// serverCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serverCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
