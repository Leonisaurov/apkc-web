#!/usr/bin/env node
// falsificación del modo de creación: el `open` del shim creaba el nodo con `nuevoNodo(a, false)` y
// tiraba el `modo` que Go ya le pasa, así que todo nacía 0644 y un `os.CreateTemp(dir, patrón)` —que
// pide 0600— salía legible por otros. Lo cazó `TestArgumentosDeFirmaNoLlevanLaPassword`
// (apkc/paquetetemporal_test.go:208) con «el fichero de la contraseña es legible por otros: 644»,
// que no era un defecto del test sino el shim mintiendo a cualquier OpenFile.
// Este guion compila una sonda fuera del repo y la corre por el shim: exige 0600 en lo que se crea
// con 0600, y que un O_CREAT sobre un fichero que ya existe no le cambie el modo.
//
// uso: node shim/prueba-modo.js        (necesita go y shim/wasm_exec.js de tu GOROOT)
//      SHIM=<copia>/navegador-sim.js node shim/prueba-modo.js   para correrlo contra otro shim
const fs = require("fs");
const os = require("os");
const path = require("path");
const { spawnSync } = require("child_process");

const dirShim = __dirname;
const shim = process.env.SHIM ? path.resolve(process.env.SHIM) : path.join(dirShim, "navegador-sim.js");
const cache = process.env.XDG_CACHE_HOME
  ? path.join(process.env.XDG_CACHE_HOME, "apkc-web")
  : path.join(os.homedir(), ".cache", "apkc-web");
const tmp = path.join(cache, "prueba-modo");

if (!fs.existsSync(path.join(path.dirname(shim), "wasm_exec.js"))) {
  console.error("falta wasm_exec.js junto al shim: cp \"$(go env GOROOT)/lib/wasm/wasm_exec.js\" shim/");
  process.exit(1);
}

// la sonda hace las cuatro formas de crear que usa apkc: CreateTemp (el caso que fallaba), un
// O_CREAT|O_EXCL con 0600, un O_CREAT sobre algo que ya está —Linux ignora el modo ahí— y un
// WriteFile con 0644. Imprime el permiso observado y deja que el guion lo compare: así el mismo
// binario sirve para el shim bueno y para el viejo.
const SONDA = `package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func permiso(calle string) string {
	info, err := os.Stat(calle)
	if err != nil {
		fmt.Println("stat:", err)
		os.Exit(2)
	}
	return fmt.Sprintf("%04o", info.Mode().Perm())
}

func main() {
	raiz, err := os.MkdirTemp("/tmp", "modo")
	if err != nil {
		fmt.Println("sin tmp:", err)
		os.Exit(2)
	}
	// 1. el caso medido: os.CreateTemp pide 0600 aunque no se diga
	t, err := os.CreateTemp(raiz, "contrasena")
	if err != nil {
		fmt.Println("createtemp:", err)
		os.Exit(2)
	}
	fmt.Println("createtemp=" + permiso(t.Name()))
	t.Close()
	// 2. O_CREAT|O_EXCL con el modo explícito
	secreto := filepath.Join(raiz, "secreto")
	f, err := os.OpenFile(secreto, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Println("open exclusivo:", err)
		os.Exit(2)
	}
	f.Close()
	fmt.Println("exclusivo=" + permiso(secreto))
	// 3. O_CREAT sobre uno que ya existe con 0666: Linux no le toca el modo
	g, err := os.OpenFile(secreto, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		fmt.Println("open sobre existente:", err)
		os.Exit(2)
	}
	g.Close()
	fmt.Println("existente=" + permiso(secreto))
	// 4. el modo normal de siempre tiene que seguir saliendo 0644 (0666 recortado por la umask 022)
	normal := filepath.Join(raiz, "normal.txt")
	if err := os.WriteFile(normal, []byte("x"), 0o644); err != nil {
		fmt.Println("write:", err)
		os.Exit(2)
	}
	fmt.Println("normal=" + permiso(normal))
	os.Exit(0)
}
`;

const entorno = { ...process.env, GOOS: "js", GOARCH: "wasm", CGO_ENABLED: "0", GOTMPDIR: path.join(tmp, "gotmp") };
fs.mkdirSync(entorno.GOTMPDIR, { recursive: true });
if (spawnSync("go", ["version"], { encoding: "utf8" }).status !== 0) {
  console.error("esta prueba compila su sonda con go, y go no está en el PATH");
  process.exit(1);
}

const dirSonda = path.join(tmp, "sonda");
fs.mkdirSync(dirSonda, { recursive: true });
fs.writeFileSync(path.join(dirSonda, "main.go"), SONDA);
const wasm = path.join(tmp, "sonda.wasm");
const compilado = spawnSync("go", ["build", "-o", wasm, "main.go"], { cwd: dirSonda, env: entorno, encoding: "utf8", timeout: 180000 });
if (compilado.status !== 0) {
  console.error("no compilé la sonda: " + ((compilado.stderr || "") + (compilado.stdout || "")));
  process.exit(1);
}

const corrida = spawnSync(process.execPath, [shim, wasm], { cwd: path.dirname(shim), encoding: "utf8", timeout: 60000 });
const salida = (corrida.stdout || "") + (corrida.stderr || "");
const fallos = [];
function comprobar(nombre, condicion, detalle) {
  if (condicion) {
    console.log(`  ok   ${nombre}`);
  } else {
    fallos.push(nombre);
    console.log(`  FALLO ${nombre}${detalle ? " — " + detalle : ""}`);
  }
}
const valor = (clave) => {
  const m = salida.match(new RegExp("^" + clave + "=([0-7]{4})$", "m"));
  return m ? m[1] : "<falta>";
};

console.log("1. lo que Go pide con 0600 nace 0600");
comprobar("rc 0 de la sonda", corrida.status === 0, `rc ${corrida.status}, salida: ${salida.trim().slice(0, 300)}`);
comprobar("CreateTemp = 0600", valor("createtemp") === "0600", valor("createtemp"));
comprobar("O_CREAT|O_EXCL con 0600 = 0600", valor("exclusivo") === "0600", valor("exclusivo"));

console.log("2. un O_CREAT sobre un fichero existente no le cambia el modo");
comprobar("sigue 0600 tras reabrirlo con 0666", valor("existente") === "0600", valor("existente"));

console.log("3. el modo de siempre no cambia");
comprobar("WriteFile con 0644 = 0644", valor("normal") === "0644", valor("normal"));

if (fallos.length) {
  console.error(`\nprueba-modo: ${fallos.length} fallo(s): ${fallos.join(", ")}`);
  process.exit(1);
}
console.log("\nprueba-modo: el shim respeta el modo de creación y no toca el de un fichero existente");
