// Package laserlabel implements the laserlabel command line interface.
package laserlabel

// Options is the top level set of command line options.
type Options struct {
	LogLevel  string `kong:"default='info',enum='debug,info,warn,error',help='log level (${enum})'"`
	LogFormat string `kong:"default='text',enum='text,json',help='log format (${enum})'"`

	Serve   serveCmd   `kong:"cmd,help='run the web UI'"`
	Gen     genCmd     `kong:"cmd,help='generate G-code'"`
	Machine machineCmd `kong:"cmd,help='talk to the machine'"`
	Fonts   fontsCmd   `kong:"cmd,help='font commands'"`
	Version versionCmd `kong:"cmd,help='print build version'"`
}
