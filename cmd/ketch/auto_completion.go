package main

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	var completionCmd = &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script",
		Long: `To load completions:

Bash:

  $ source <(ketch completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ ketch completion bash > /etc/bash_completion.d/ketch
  # macOS:
  $ ketch completion bash > /usr/local/etc/bash_completion.d/ketch

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ ketch completion zsh > "${fpath[1]}/_ketch"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ ketch completion fish | source

  # To load completions for each session, execute once:
  $ ketch completion fish > ~/.config/fish/completions/ketch.fish

PowerShell:

  PS> ketch completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> ketch completion powershell > ketch.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				cmd.Root().GenPowerShellCompletion(os.Stdout)
			}
		},
	}
	return completionCmd
}

func autoCompleteAppNames(cfg config, toComplete ...string) ([]string, cobra.ShellCompDirective) {
	names, err := appListNames(cfg, toComplete...)
	if err != nil {
		return []string{err.Error()}, cobra.ShellCompDirectiveError
	}
	return names, cobra.ShellCompDirectiveNoSpace
}

func autoCompleteFrameworkNames(cfg config, toComplete ...string) ([]string, cobra.ShellCompDirective) {
	names, err := frameworkListNames(cfg, toComplete...)
	if err != nil {
		return []string{err.Error()}, cobra.ShellCompDirectiveError
	}
	return names, cobra.ShellCompDirectiveNoSpace
}

func autoCompleteBuilderNames(cfg config, toComplete ...string) ([]string, cobra.ShellCompDirective) {
	return builderList.Names(toComplete...), cobra.ShellCompDirectiveNoSpace
}

func autoCompleteJobNames(cfg config, toComplete ...string) ([]string, cobra.ShellCompDirective) {
	names, err := jobListNames(cfg, toComplete...)
	if err != nil {
		return []string{err.Error()}, cobra.ShellCompDirectiveError
	}
	return names, cobra.ShellCompDirectiveNoSpace
}
