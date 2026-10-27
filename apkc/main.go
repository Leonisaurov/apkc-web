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

	findSDKs()

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
version - print the commit this binary was built from
`)
}
