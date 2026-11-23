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
	case "dex", "link", "compile", "check", "verify", "keystore", "rotate", "debug-keystore", "create-keystore", "preview":
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
	case "keystore":
		keystore()
	case "debug-keystore":
		comandoDebugKeystore()
	case "create-keystore":
		comandoCreateKeystore()
	case "rotate":
		rotar()
	case "dex":
		dex()
	case "compile":
		compilar()
	case "check":
		check()
	case "link":
		link()
	case "verify":
		verificar()
	case "preview":
		preview()
	case "version":
		// what a script asks when it has to know which apkc answered the rest
		fmt.Println(buildStamp())
	}

	// preview es el único comando cuyo stdout es el contrato entero: la línea de JSON del modelo.
	// La línea de cierre de los demás sería un segundo documento en la misma salida y rompería a
	// quien lea stdout tal cual (`apkc preview app.apk | jq .`), así que en preview va a stderr.
	if os.Args[1] == "preview" {
		fmt.Fprintln(os.Stderr, "[apkc] finished in", time.Since(start))
	} else {
		LogV("apkc", "finished in", time.Since(start))
	}
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
sign   - put an existing apk under a v1+v2+v3 signature of its own, with no zipalign, no
         apksigner and no external program at all (--keystore, --storepass,
         --keyalias, --out; without --out the package is signed in place). An .aab gets
         the v1 JAR signature only, because a bundle has no APK Signing Block
keystore - list the signing identities of a keystore (alias, subject, SHA-256 and validity)
         as one line of JSON, with no process and no JVM (--keystore, --storepass)
debug-keystore - write the typical JKS debug key (androiddebugkey / android / CN=Android
         Debug) with no process and no JVM (--out, --storepass, --keyalias, --validity-days)
create-keystore - write a JKS release key with a custom subject, no process and no JVM
         (--out, --storepass, --keyalias, --common-name, --organization, --country,
         --validity-days, --force)
rotate - write the SigningCertificateLineage linking an old key to a new one, in the format
         apksigner --lineage reads (--old-keystore/--old-storepass/--old-keyalias,
         --new-keystore/--new-storepass/--new-keyalias, --out)
dex    - turn compiled .class files into a classes.dex in Go, with no d8 and no subprocess
         (-o salida, y una o mas carpetas de entrada; es la etapa que puede correr en el
         navegador, el resto del build todavia necesita aapt2 y javac)
verify - read one package back and check the signatures this tool writes: the v1 manifest
         and .SF digests recomputed over the stored bytes, the PKCS#7 over the .SF, the v2
         block before the central directory, and the v4 .idsig next to the apk (verity tree,
         digest and RSA signature). An .aab only has v1 and is reported as such. No apksigner,
         no jarsigner, no subprocess - which is what makes it usable inside the tab. What is
         not measured here is refused by name (DSA, SHA-1, v3, several signers), and the trust
         chain is never validated: a debug certificate is self-signed and there is no anchor.
check  - read one .java file and print, as one line of JSON on stdout, the errors the parser and
         the emitter find, without writing any .class (--android-jar to resolve names; without a
         jar only the parser is checked). --proyecto links the project's resources in memory so
         that R.layout.main resolves instead of being reported as an unknown name and compiles the
         other .java files of src/ and gen/ together with the requested one, so its classes resolve;
         only the requested file is reported, and every error comes with archivo:linea:columna. No
         process is started, so it runs inside the tab.
preview - read one apk and print, as one line of JSON on stdout, the model of its first screen:
         type, id, text, children and an approximate rectangle per node. The layout is the one
         the app's onCreate passes to setContentView (the Dalvik interpreter of Phase 2 reads it
         from classes.dex). The interpreter runs onCreate and does NOT click anything on its own:
         each --pulsar id runs that view's OnClickListener, in order and inside the same run, and
         the text the app writes with TextView.setText travels in ejecucion.capturas and in each
         node's textoDinamico; the app's own output (println, Log.*) travels in ejecucion.consola.
         --sin-ejecutar keeps Phase 1's model. No process is started either.
version - print the commit this binary was built from
`)
}
