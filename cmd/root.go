package cmd

import "github.com/spf13/cobra"

var (
	RootCmd = &cobra.Command{
		Use:   "emulifx",
		Short: "Emulate a LIFX bulb on the network",
	}

	// Flags.

	addr  string
	label string
	group string
)

func init() {
	RootCmd.PersistentFlags().StringVarP(&addr, "addr", "a", "127.0.0.1:0",
		"the address to bind to for receiving messages from devices on the network")
	RootCmd.PersistentFlags().StringVarP(&label, "label", "l", "MyDevice",
		"the initial label of the device")
	RootCmd.PersistentFlags().StringVarP(&group, "group", "g", "MyGroup",
		"the initial group of the device")
}
