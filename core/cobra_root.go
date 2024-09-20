package core

import (
	"fmt"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"io"
	"os"
	"path/filepath"
)

var (
	rootModified    bool
	shellFlag       bool
	quietFlag       bool
	debugFlag       bool
	radDebugFlag    bool
	stdinScriptName string
	mockResponses   MockResponseSlice
)

type CmdInput struct {
	RIo   *RadIo
	RExit *func(int)
	RReq  *Requester
}

func NewRootCmd(cmdInput CmdInput) *cobra.Command {
	if cmdInput.RIo == nil {
		RIo = RadIo{
			StdIn:  os.Stdin,
			StdOut: os.Stdout,
			StdErr: os.Stderr,
		}
	} else {
		RIo = *cmdInput.RIo
	}
	if cmdInput.RExit == nil {
		RExit = os.Exit
	} else {
		RExit = *cmdInput.RExit
	}
	if cmdInput.RReq == nil {
		RReq = NewRequester()
	} else {
		RReq = cmdInput.RReq
	}
	rootModified = false

	return &cobra.Command{
		Use:     "",
		Short:   "Request And Display (RAD)",
		Long:    `Request And Display (RAD): A tool for making HTTP requests, extracting details, and displaying the result.`,
		Version: "0.2.8",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if !rootModified {
				RP = NewPrinter(cmd, shellFlag, quietFlag, debugFlag, radDebugFlag)
				RP.RadDebug(fmt.Sprintf("Args passed: %v\n", args))
				if radDebugFlag {
					cmd.Flags().VisitAll(func(flag *pflag.Flag) {
						RP.RadDebug(fmt.Sprintf("Flag %s: %v\n", flag.Name, flag.Value))
					})
				}
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			for _, mockResponse := range mockResponses {
				RReq.AddMockedResponse(mockResponse.Pattern, mockResponse.FilePath)
				RP.RadDebug(fmt.Sprintf("Mock response added: %q -> %q\n", mockResponse.Pattern, mockResponse.FilePath))
			}

			var scriptName string
			var rslSourceCode string
			if stdinScriptName != "" {
				// we're in stdin mode
				scriptName = filepath.Base(stdinScriptName)
				source, err := io.ReadAll(RIo.StdIn)
				if err != nil {
					RP.RadErrorExit(fmt.Sprintf("Could not read from stdin: %v\n", err))
				}
				rslSourceCode = string(source)
			} else if len(args) == 0 {
				cmd.Help()
				return
			} else {
				scriptPath := args[0]
				scriptName = filepath.Base(scriptPath)
				rslSourceCode = readSource(scriptPath)
			}

			if rootModified {
				return
			}

			extractMetadataAndModifyCmd(cmd, scriptName, rslSourceCode)
			cmd.Execute()
		},
	}
}

func extractMetadataAndModifyCmd(cmd *cobra.Command, scriptName string, rslSourceCode string) {
	l := NewLexer(RP, rslSourceCode)
	l.Lex()

	p := NewParser(RP, l.Tokens)
	instructions := p.Parse()

	scriptMetadata := ExtractMetadata(instructions)
	modifyCmd(cmd, scriptName, scriptMetadata, instructions)
	rootModified = true
}

func modifyCmd(cmd *cobra.Command, scriptName string, scriptMetadata ScriptMetadata, instructions []Stmt) {
	useString := GenerateUseString(scriptName, scriptMetadata.Args)
	var cobraArgs []*CobraArg
	cmd.Use = useString
	cmd.Short = ShortDescription(scriptMetadata)
	cmd.Long = LongDescription(scriptMetadata)
	cmd.FParseErrWhitelist = cobra.FParseErrWhitelist{} // re-enable erroring on unknown flags. note: maybe remove for 'catchall' args?
	cmd.Run = func(cmd *cobra.Command, args []string) {
		// fill in positional args, and
		// error if required args are missing
		posArgsIndex := 0
		if stdinScriptName == "" {
			// We're invoked on an actual string path, which will be the first arg. Ignore.
			posArgsIndex = 1
		}
		var missingArgs []string
		shouldPrintHelp := cobraArgs != nil
		for _, cobraArg := range cobraArgs {
			argName := cobraArg.Arg.ApiName
			cobraFlag := cmd.Flags().Lookup(argName)
			if !cobraFlag.Changed {
				// flag has not been explicitly set by the user
				if posArgsIndex < len(args) {
					// there's a positional arg to fill it
					cobraArg.SetValue(args[posArgsIndex])
					posArgsIndex++
					shouldPrintHelp = false
				} else if cobraArg.Arg.IsOptional {
					// there's no positional arg to fill it, but that's okay because it's optional, so continue
					// but first, fill in the optional's default value if it exists
					cobraArg.InitializeOptional()
					shouldPrintHelp = false
					continue
				} else if cobraArg.IsBool() {
					// all bools are implicitly optional and default false, unless explicitly defaulted to true
					// this branch implies it was not defaulted to true
					cobraArg.SetValue("false")
					shouldPrintHelp = false
				} else {
					missingArgs = append(missingArgs, argName)
				}
			} else {
				shouldPrintHelp = false
			}
		}

		if shouldPrintHelp {
			cmd.Help()
			return
		}

		if len(missingArgs) > 0 {
			RP.UsageErrorExit(fmt.Sprintf("Missing required arguments: %s\n", missingArgs))
		}

		// error if not all positional args were used
		if posArgsIndex < len(args) {
			RP.UsageErrorExit(fmt.Sprintf("Too many positional arguments. Unused: %v\n", args[posArgsIndex:]))
		}

		interpreter := NewInterpreter(instructions)
		interpreter.InitArgs(cobraArgs)
		interpreter.Run()

		if shellFlag {
			env := interpreter.env
			for varName, val := range env.Vars {
				// todo handle different types specifically
				RP.PrintForShellEval(fmt.Sprintf("export %s=\"%v\"\n", varName, val.value))
			}
		}
	}

	for _, arg := range scriptMetadata.Args {
		cobraArg := CreateCobraArg(RP, cmd, arg)
		cobraArgs = append(cobraArgs, &cobraArg)
	}

	// hide global flags, that distract from the particular script
	cmd.Flags().MarkHidden("version")
	cmd.Flags().MarkHidden("help")
	cmd.PersistentFlags().MarkHidden("SHELL")
	cmd.PersistentFlags().MarkHidden("STDIN")
	cmd.PersistentFlags().MarkHidden("QUIET")
	cmd.PersistentFlags().MarkHidden("DEBUG")
	cmd.PersistentFlags().MarkHidden("RAD-DEBUG")
	cmd.PersistentFlags().MarkHidden("MOCK-RESPONSE")
}

func readSource(scriptPath string) string {
	source, err := os.ReadFile(scriptPath)
	if err != nil {
		RP.RadErrorExit(fmt.Sprintf("Could not read script '%s': %v\n", scriptPath, err))
		RExit(1)
	}
	return string(source)
}

func InitCmd(cmd *cobra.Command) {
	// this is a bit hacky, but bear with me!
	// we intercept the very first call to help, it implies that the user has set the help flag
	// however, we won't yet have read the RSL script if it was also provided, so we may first
	// try to read that, register the args, etc, that's relevant to help, and then re-run the help
	// command, so that it can print help for the script
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// immediately reset the help func, as we only want this hacked
		// version to run once
		cmd.SetHelpFunc(nil)

		// try to detect if help has been called on either a script or with --STDIN flag
		if len(args) >= 2 {
			if lo.Some(args[1:], []string{"-h", "--help"}) && stdinScriptName == "" {
				// it has, and with a rsl file source, so let's modify the cmd and re-run the root again
				scriptPath := args[0]
				rslSourceCode := readSource(scriptPath)
				extractMetadataAndModifyCmd(cmd, filepath.Base(scriptPath), rslSourceCode)
			} else if stdinScriptName != "" {
				// it has, and with reading rsl from stdin, so let's modify the cmd and re-run the root again
				source, err := io.ReadAll(RIo.StdIn)
				if err == nil {
					extractMetadataAndModifyCmd(cmd, filepath.Base(stdinScriptName), string(source))
				} else {
					RP.RadErrorExit(fmt.Sprintf("Could not read from stdin: %v\n", err))
				}
			}
		}

		cmd.Help()

		if shellFlag && stdinScriptName != "" {
			// if both these flags are set, we're likely being invoked from within a bash script, so let's
			// output an exit 0 for bash to eval and exit, so it doesn't continue
			RP.PrintForShellEval("exit 0")
		}
	})

	// global flags
	// todo think more about bash vs. shell
	cmd.PersistentFlags().BoolVar(&shellFlag, "SHELL", false, "Outputs shell/bash exports of variables, so they can be eval'd")
	cmd.PersistentFlags().StringVar(&stdinScriptName, "STDIN", "", "Enables reading RSL from stdin, and takes a string arg to be treated as the 'script name', usually $0")
	cmd.PersistentFlags().BoolVar(&quietFlag, "QUIET", false, "Suppresses some output.")
	cmd.PersistentFlags().BoolVar(&debugFlag, "DEBUG", false, "Enables debug output. Intended for RSL script developers.")
	cmd.PersistentFlags().BoolVar(&radDebugFlag, "RAD-DEBUG", false, "Enables Rad debug output. Intended for Rad developers.")
	// todo help prints as `--MOCK-RESPONSE mockResponse` which is not ideal
	cmd.PersistentFlags().Var(&mockResponses, "MOCK-RESPONSE", "Add mock response for json requests (pattern:filePath)")
	cmd.SetOut(RIo.StdErr)
}

func Execute() {
	rootCmd := NewRootCmd(CmdInput{})
	InitCmd(rootCmd)
	err := rootCmd.Execute()
	if err != nil {
		RExit(1)
	}
}
