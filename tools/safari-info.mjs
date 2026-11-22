// Identidad del motor que corre en el job `safari` de GitHub Actions. El dato es el ejecutable
// de Playwright y la presencia de Safari.app. No afirma "Safari real" sin evidencia: Playwright
// empaqueta WebKit, que es el mismo motor pero no la aplicación de Apple.
// Uso: node tools/safari-info.mjs
import fs from "node:fs";

let webkit;
try {
  ({ webkit } = await import("playwright"));
} catch (e) {
  console.log("playwright: no instalado (nada que medir)");
  process.exit(0);
}

const ejecutable = webkit.executablePath();
const safariApp = fs.existsSync("/Applications/Safari.app");
let version = "n/d";
let userAgent = "n/d";
let navegador = null;
try {
  navegador = await webkit.launch({ headless: true });
  version = navegador.version();
  const pagina = await navegador.newPage();
  userAgent = await pagina.evaluate(() => navigator.userAgent);
} catch (e) {
  console.log("lanzamiento WebKit: falló — " + (e && e.message ? e.message.split("\n")[0] : String(e)));
} finally {
  if (navegador) await navegador.close();
}

const esPlaywright = /ms-playwright|playwright/i.test(ejecutable);
console.log("motor: WebKit (Playwright)");
console.log("ejecutable: " + ejecutable);
console.log("de Playwright: " + (esPlaywright ? "si" : "no"));
console.log("Safari.app instalado: " + (safariApp ? "si" : "no"));
console.log("version: " + version);
console.log("userAgent: " + userAgent);
console.log("es Safari real: " + (safariApp && !esPlaywright ? "podria serlo, requiere mas evidencia" : "no"));
console.log("nota: Playwright no automatiza Safari.app; ejecuta su propio WebKit en macOS.");
