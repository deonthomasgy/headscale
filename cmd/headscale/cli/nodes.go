package cli

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/juanfont/headscale"
	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
	"inet.af/netaddr"
	"tailscale.com/types/key"
)

var availableColumns = map[string]string{
	"id":           "ID",
	"name":         "Names",
	"nodekey":      "NodeKey",
	"namespace":    "Namespace",
	"ip_addresses": "IP addresses",
	"ephemeral":    "Ephemeral",
	"last_seen":    "Last seen",
	"online":       "Online",
	"expired":      "Expired",
	"tags":         "Tags",
	"routes":       "Routes",
}

var defaultColumns = []string{
	"id",
	"name",
	"nodekey",
	"namespace",
	"ip_addresses",
	"ephemeral",
	"last_seen",
	"online",
	"expired",
}

func init() {
	rootCmd.AddCommand(nodeCmd)
	listNodesCmd.Flags().StringP("namespace", "n", "", "Filter by namespace")
	nodeCmd.AddCommand(listNodesCmd)

	listNodesCmd.Flags().StringSliceP("columns", "", defaultColumns, "Customize layout by listing columns")
	nodeCmd.AddCommand(listNodesCmd)

	registerNodeCmd.Flags().StringP("namespace", "n", "", "Namespace")
	err := registerNodeCmd.MarkFlagRequired("namespace")
	if err != nil {
		log.Fatalf(err.Error())
	}
	registerNodeCmd.Flags().StringP("key", "k", "", "Key")
	err = registerNodeCmd.MarkFlagRequired("key")
	if err != nil {
		log.Fatalf(err.Error())
	}
	nodeCmd.AddCommand(registerNodeCmd)

	expireNodeCmd.Flags().Uint64P("identifier", "i", 0, "Node identifier (ID)")
	err = expireNodeCmd.MarkFlagRequired("identifier")
	if err != nil {
		log.Fatalf(err.Error())
	}
	nodeCmd.AddCommand(expireNodeCmd)

	deleteNodeCmd.Flags().Uint64P("identifier", "i", 0, "Node identifier (ID)")
	err = deleteNodeCmd.MarkFlagRequired("identifier")
	if err != nil {
		log.Fatalf(err.Error())
	}
	nodeCmd.AddCommand(deleteNodeCmd)

	moveNodeCmd.Flags().Uint64P("identifier", "i", 0, "Node identifier (ID)")

	err = moveNodeCmd.MarkFlagRequired("identifier")
	if err != nil {
		log.Fatalf(err.Error())
	}

	moveNodeCmd.Flags().StringP("namespace", "n", "", "New namespace")

	err = moveNodeCmd.MarkFlagRequired("namespace")
	if err != nil {
		log.Fatalf(err.Error())
	}
	nodeCmd.AddCommand(moveNodeCmd)
}

var nodeCmd = &cobra.Command{
	Use:     "nodes",
	Short:   "Manage the nodes of Headscale",
	Aliases: []string{"node", "machine", "machines"},
}

var registerNodeCmd = &cobra.Command{
	Use:   "register",
	Short: "Registers a machine to your network",
	Run: func(cmd *cobra.Command, args []string) {
		output, _ := cmd.Flags().GetString("output")
		namespace, err := cmd.Flags().GetString("namespace")
		if err != nil {
			ErrorOutput(err, fmt.Sprintf("Error getting namespace: %s", err), output)

			return
		}

		ctx, client, conn, cancel := getHeadscaleCLIClient()
		defer cancel()
		defer conn.Close()

		machineKey, err := cmd.Flags().GetString("key")
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf("Error getting machine key from flag: %s", err),
				output,
			)

			return
		}

		request := &v1.RegisterMachineRequest{
			Key:       machineKey,
			Namespace: namespace,
		}

		response, err := client.RegisterMachine(ctx, request)
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf(
					"Cannot register machine: %s\n",
					status.Convert(err).Message(),
				),
				output,
			)

			return
		}

		SuccessOutput(response.Machine, "Machine register", output)
	},
}

var listNodesCmd = &cobra.Command{
	Use:     "list",
	Short:   "List nodes",
	Aliases: []string{"ls", "show"},
	Run: func(cmd *cobra.Command, args []string) {
		output, _ := cmd.Flags().GetString("output")
		namespace, err := cmd.Flags().GetString("namespace")
		if err != nil {
			ErrorOutput(err, fmt.Sprintf("Error getting namespace: %s", err), output)

			return
		}

		columns, err := cmd.Flags().GetStringSlice("columns")
		if err != nil {
			ErrorOutput(err, fmt.Sprintf("Error getting columns: %s", err), output)

			return
		}

		ctx, client, conn, cancel := getHeadscaleCLIClient()
		defer cancel()
		defer conn.Close()

		request := &v1.ListMachinesRequest{
			Namespace: namespace,
		}

		response, err := client.ListMachines(ctx, request)
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf("Cannot get nodes: %s", status.Convert(err).Message()),
				output,
			)

			return
		}

		if output != "" {
			SuccessOutput(response.Machines, "", output)

			return
		}

		tableData, err := nodesToPtables(namespace, columns, response.Machines)
		if err != nil {
			ErrorOutput(err, fmt.Sprintf("Error converting to table: %s", err), output)

			return
		}

		err = pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf("Failed to render pterm table: %s", err),
				output,
			)

			return
		}
	},
}

var expireNodeCmd = &cobra.Command{
	Use:     "expire",
	Short:   "Expire (log out) a machine in your network",
	Long:    "Expiring a node will keep the node in the database and force it to reauthenticate.",
	Aliases: []string{"logout", "exp", "e"},
	Run: func(cmd *cobra.Command, args []string) {
		output, _ := cmd.Flags().GetString("output")

		identifier, err := cmd.Flags().GetUint64("identifier")
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf("Error converting ID to integer: %s", err),
				output,
			)

			return
		}

		ctx, client, conn, cancel := getHeadscaleCLIClient()
		defer cancel()
		defer conn.Close()

		request := &v1.ExpireMachineRequest{
			MachineId: identifier,
		}

		response, err := client.ExpireMachine(ctx, request)
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf(
					"Cannot expire machine: %s\n",
					status.Convert(err).Message(),
				),
				output,
			)

			return
		}

		SuccessOutput(response.Machine, "Machine expired", output)
	},
}

var deleteNodeCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a node",
	Aliases: []string{"del"},
	Run: func(cmd *cobra.Command, args []string) {
		output, _ := cmd.Flags().GetString("output")

		identifier, err := cmd.Flags().GetUint64("identifier")
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf("Error converting ID to integer: %s", err),
				output,
			)

			return
		}

		ctx, client, conn, cancel := getHeadscaleCLIClient()
		defer cancel()
		defer conn.Close()

		getRequest := &v1.GetMachineRequest{
			MachineId: identifier,
		}

		getResponse, err := client.GetMachine(ctx, getRequest)
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf(
					"Error getting node node: %s",
					status.Convert(err).Message(),
				),
				output,
			)

			return
		}

		deleteRequest := &v1.DeleteMachineRequest{
			MachineId: identifier,
		}

		confirm := false
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			prompt := &survey.Confirm{
				Message: fmt.Sprintf(
					"Do you want to remove the node %s?",
					getResponse.GetMachine().Name,
				),
			}
			err = survey.AskOne(prompt, &confirm)
			if err != nil {
				return
			}
		}

		if confirm || force {
			response, err := client.DeleteMachine(ctx, deleteRequest)
			if output != "" {
				SuccessOutput(response, "", output)

				return
			}
			if err != nil {
				ErrorOutput(
					err,
					fmt.Sprintf(
						"Error deleting node: %s",
						status.Convert(err).Message(),
					),
					output,
				)

				return
			}
			SuccessOutput(
				map[string]string{"Result": "Node deleted"},
				"Node deleted",
				output,
			)
		} else {
			SuccessOutput(map[string]string{"Result": "Node not deleted"}, "Node not deleted", output)
		}
	},
}

var moveNodeCmd = &cobra.Command{
	Use:     "move",
	Short:   "Move node to another namespace",
	Aliases: []string{"mv"},
	Run: func(cmd *cobra.Command, args []string) {
		output, _ := cmd.Flags().GetString("output")

		identifier, err := cmd.Flags().GetUint64("identifier")
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf("Error converting ID to integer: %s", err),
				output,
			)

			return
		}

		namespace, err := cmd.Flags().GetString("namespace")
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf("Error getting namespace: %s", err),
				output,
			)

			return
		}

		ctx, client, conn, cancel := getHeadscaleCLIClient()
		defer cancel()
		defer conn.Close()

		getRequest := &v1.GetMachineRequest{
			MachineId: identifier,
		}

		_, err = client.GetMachine(ctx, getRequest)
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf(
					"Error getting node: %s",
					status.Convert(err).Message(),
				),
				output,
			)

			return
		}

		moveRequest := &v1.MoveMachineRequest{
			MachineId: identifier,
			Namespace: namespace,
		}

		moveResponse, err := client.MoveMachine(ctx, moveRequest)
		if err != nil {
			ErrorOutput(
				err,
				fmt.Sprintf(
					"Error moving node: %s",
					status.Convert(err).Message(),
				),
				output,
			)

			return
		}

		SuccessOutput(moveResponse.Machine, "Node moved to another namespace", output)
	},
}

func nodesToPtables(
	currentNamespace string,
	withColumns []string,
	machines []*v1.Machine,
) (pterm.TableData, error) {
	var tableHeader []string

	if len(withColumns) > 0 {
		for _, column := range withColumns {
			tableHeader = append(tableHeader, availableColumns[column])
		}
	} else {
		for _, column := range defaultColumns {
			tableHeader = append(tableHeader, availableColumns[column])
		}
	}

	tableData := pterm.TableData{tableHeader}

	for _, machine := range machines {
		var ephemeral bool
		if machine.PreAuthKey != nil && machine.PreAuthKey.Ephemeral {
			ephemeral = true
		}

		var lastSeen time.Time
		var lastSeenTime string
		if machine.LastSeen != nil {
			lastSeen = machine.LastSeen.AsTime()
			lastSeenTime = lastSeen.Format("2006-01-02 15:04:05")
		}

		var expiry time.Time
		if machine.Expiry != nil {
			expiry = machine.Expiry.AsTime()
		}

		var nodeKey key.NodePublic
		err := nodeKey.UnmarshalText(
			[]byte(headscale.NodePublicKeyEnsurePrefix(machine.NodeKey)),
		)
		if err != nil {
			return nil, err
		}

		var online string
		if lastSeen.After(
			time.Now().Add(-5 * time.Minute),
		) { // TODO: Find a better way to reliably show if online
			online = pterm.LightGreen("online")
		} else {
			online = pterm.LightRed("offline")
		}

		var expired string
		if expiry.IsZero() || expiry.After(time.Now()) {
			expired = pterm.LightGreen("no")
		} else {
			expired = pterm.LightRed("yes")
		}

		var namespace string
		if currentNamespace == "" || (currentNamespace == machine.Namespace.Name) {
			namespace = pterm.LightMagenta(machine.Namespace.Name)
		} else {
			// Shared into this namespace
			namespace = pterm.LightYellow(machine.Namespace.Name)
		}

		var IpV4Address string
		var IpV6Address string
		for _, addr := range machine.IpAddresses {
			if netaddr.MustParseIP(addr).Is4() {
				IpV4Address = addr
			} else {
				IpV6Address = addr
			}
		}

		var routes []string
		for _, route := range machine.RequestedRoutes {
			if isStringInSlice(machine.EnabledRoutes, route) {
				routes = append(routes, "*"+pterm.LightGreen(route))
			} else {
				routes = append(routes, pterm.LightRed(route))
			}
		}

		defaultData := map[string]string{
			"id":           strconv.FormatUint(machine.Id, headscale.Base10),
			"name":         machine.Name,
			"nodekey":      nodeKey.ShortString(),
			"namespace":    namespace,
			"ip_addresses": strings.Join([]string{IpV4Address, IpV6Address}, ", "),
			"ephemeral":    strconv.FormatBool(ephemeral),
			"last_seen":    lastSeenTime,
			"online":       online,
			"expired":      expired,
			"tags":         strings.Join(machine.RequestTags, ", "),
			"routes":       strings.Join(routes, ", "),
		}

		var nodeData []string
		if len(withColumns) > 0 {
			for _, column := range withColumns {
				nodeData = append(nodeData, defaultData[column])
			}
		} else {
			for _, column := range defaultColumns {
				nodeData = append(nodeData, defaultData[column])
			}
		}

		tableData = append(tableData, nodeData)
	}

	return tableData, nil
}
