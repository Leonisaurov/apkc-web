package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	start := time.Now()

	if len(os.Args) < 2 {
		printHelp()
		return
	}

	// findSDKs probes the SDK and the JDK to set the paths the stages shell out to. The
	// commands this fork runs without a subprocess anywhere -dex, link, compile and
	// verify- do not need that probe, and inside a browser-shaped host there is no SDK to
	// find: the probe used to end the command before it started.
	// verify reads one package and starts nothing either, for the same reason: it is the
	// stage the tab needs to say something about the apk it just wrote.
	switch os.Args[1] {
	case "dex", "link", "compile", "verify":
	case "build":
		// `apkc build --linker go --compiler go` starts no process either: it is the whole build of
		// the tab, and there is no SDK to find in there (§44)
		if !sinProcesos(os.Args[2:]) {
			findSDKs()
		}
	default:
		findSDKs()
	}

	switch os.Args[1] {
	case "doctor":
		doctor()
	case "create":
		create()
	case "build":
		build()
	case "run":
		run()
	case "watch":
		watch()
	case "clean":
		clean()
	case "sign":
		sign()
	case "dex":
		dex()
	case "compile":
		compilar()
	case "link":
		link()
	case "verify":
		verificar()
	case "version":
		// what a script asks when it has to know which apkc answered the rest
		fmt.Println(buildStamp())
	}

	LogV("apkc", "finished in", time.Since(start))
}

func printHelp() {
	fmt.Print(`
commands:

doctor - check if Android sdk is accessible
create - create a new project
build  - build apk (--inc for an incremental build, --aab for an app bundle,
         --core-library to let d8 dex java.* classes, --signer jvm|go|auto)
run    - build apk and run it on an available device (--inc, --rish)
watch  - rebuild on every change (--inc, --run installs and follows the app)
clean  - delete build/ dir
sign   - put an existing apk under a v1+v2 signature of its own, with no zipalign, no
         apksigner and no external program at all (--keystore, --storepass,
         --keyalias, --out; without --out the package is signed in place)
dex    - turn compiled .class files into a classes.dex in Go, with no d8 and no subprocess
         (-o salida, y una o mas carpetas de entrada; es la etapa que puede correr en el
         navegador, el resto del build todavia necesita aapt2 y javac)
verify - read one package back and check the two signatures this tool writes: the v1 manifest
         and .SF digests recomputed over the stored bytes, the PKCS#7 over the .SF, and the v2
         block before the central directory with its content digest recalculated. No apksigner,
         no jarsigner, no subprocess - which is what makes it usable inside the tab. What is not
         measured here is refused by name (DSA, SHA-1, v3, v4, several signers), and the trust
         chain is never validated: a debug certificate is self-signed and there is no anchor.
version - print the commit this binary was built from
`)
}
