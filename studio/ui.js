// pegado a la página: lee el formulario, llama a lo que está en app.js y guarda lo anotado

const form = document.getElementById("form");
const estadoZip = document.getElementById("estado-zip");

// los puntos donde se rompe una primera vez, contados en la propia página
const STUMBLES = [
  "no encuentro el SDK o el JDK que ve `apkc doctor`",
  "el keystore de depuración no existe todavía",
  "no sé qué tiene que haber en `jar/` para usar una librería",
  "un `.aar` no se parece a lo que apkc lee",
  "Kotlin: falta el compilador o la stdlib",
  "el teléfono: adb, o Shizuku para `--rish`",
  "no sé qué flag usar (`--inc`, `--signer`, `--aab`)",
  "no entiendo un aviso que sale en cada ronda",
  "tardó más de lo que esperaba y no sé qué etapa fue",
];

function estado() {
  const f = new FormData(form);
  return {
    name: f.get("name") || "Mi app",
    package: (f.get("package") || "com.mi.app").toLowerCase(),
    minsdk: f.get("minsdk") || "21",
    targetsdk: f.get("targetsdk") || "33",
    hello: f.get("hello") || "Hello World!",
    button: f.has("button"),
    buttonLabel: f.get("buttonLabel") || "Press me",
    toast: f.get("toast") || "Clicked!",
    kotlin: f.has("kotlin"),
    libs: f.has("libs"),
    native: f.has("native"),
    assets: f.has("assets"),
    device: f.has("device"),
  };
}

function escribir() {
  const s = estado();
  const p = buildProject(s);
  const rutas = [...p.dirs.map((d) => d + "/"), ...Object.keys(p.files)].sort();
  document.getElementById("arbol").textContent = s.package + "/\n" +
    rutas.map((r) => "  " + r).join("\n");

  const cont = document.getElementById("ficheros");
  cont.textContent = "";
  for (const [ruta, texto] of Object.entries(p.files)) {
    const d = document.createElement("details");
    const sum = document.createElement("summary");
    sum.textContent = ruta;
    const pre = document.createElement("pre");
    pre.textContent = texto;
    d.append(sum, pre);
    cont.append(d);
  }

  const cmd = ["# 0. falta el SDK y un JDK, y que apkc esté en el PATH",
               "apkc doctor", "",
               "# 1. andamiaje (esta página puede saltárselo: el zip ya lo trae hecho)",
               "apkc create ./" + s.package.split(".").pop(), "",
               "# 2. echar encima los ficheros generados aquí (el zip tiene los mismos nombres)",
               "# 3. compilar", "apkc build --inc", "",
               "# 4. instalar y arrancar",
               s.device ? "apkc run --rish   # con Shizuku arriba; si no, `apkc run` a secas usa adb"
                        : "adb install -r build/app.apk"]
    .join("\n");
  document.getElementById("comandos").textContent = cmd;
}

function nombreZip() {
  const s = estado();
  return "apkc-" + s.package.replace(/[^a-z0-9.]+/g, "-") + ".zip";
}

function bajar() {
  const s = estado();
  const p = buildProject(s);
  const entradas = Object.entries(p.files).map(([ruta, texto]) => [s.package + "/" + ruta, texto]);
  // los directorios vacíos tienen que ir o el descompresor no los crea: lib/arm64-v8a y
  // compania son parte de lo que apkc espera encontrar
  for (const d of p.dirs) entradas.push([s.package + "/" + d + "/", ""]);
  const blob = new Blob([makeZip(entradas)], { type: "application/zip" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = nombreZip();
  a.click();
  URL.revokeObjectURL(a.href);
  estadoZip.textContent = "descargado " + nombreZip();
  registrar("descarga", s.package);
}

function analizarPegado() {
  const texto = document.getElementById("log").value;
  const { hits, desconocidas } = analyze(texto);
  const salida = document.getElementById("salida-analisis");
  salida.textContent = "";

  if (!texto.trim()) {
    const p = document.createElement("p");
    p.className = "nota";
    p.textContent = "Pega arriba la salida de un comando; sin texto no hay nada que traducir.";
    salida.append(p);
    return;
  }

  const orden = ["entorno", "proyecto", "herramienta", "dispositivo", "informativo"];
  const grupos = {};
  for (const h of hits) (grupos[h.capa] ||= []).push(h);
  for (const capa of orden) {
    if (!grupos[capa]) continue;
    const h2 = document.createElement("h3");
    h2.textContent = capa + " (" + grupos[capa].length + ")";
    salida.append(h2);
    for (const g of grupos[capa]) {
      const d = document.createElement("div");
      d.className = "regla";
      d.innerHTML = "<strong>" + g.que + "</strong><p>" + g.hacer +
        "</p><code>" + g.linea.replace(/</g, "&lt;") + "</code>";
      salida.append(d);
      registrar("regla", g.id);
    }
  }
  if (desconocidas.length) {
    const h2 = document.createElement("h3");
    h2.textContent = "líneas que la página no conoce (" + desconocidas.length + ")";
    const pre = document.createElement("pre");
    pre.textContent = desconocidas.join("\n");
    const nota = document.createElement("p");
    nota.className = "nota";
    nota.textContent = "Esto es lo que el analizador todavía no sabe explicar. Vale más que " +
      "cualquier regla: es una frase de apkc que alguien leyó y no entendió.";
    salida.append(h2, nota, pre);
    registrar("desconocidas", String(desconocidas.length));
  }
  if (!hits.length && !desconocidas.length) {
    const p = document.createElement("p");
    p.className = "nota";
    p.textContent = "No hay ni una línea reconocible de apkc en ese texto.";
    salida.append(p);
  }
}

function pintarFriccion() {
  const ul = document.getElementById("friccion");
  const l = loadLog();
  for (const s of STUMBLES) {
    const li = document.createElement("li");
    const id = "stumble:" + s;
    li.innerHTML = '<label><input type="checkbox" data-stumble="' + id + '"' +
      (l.pasos.includes(id) ? " checked" : "") + "> " + s + "</label>";
    ul.append(li);
  }
  ul.addEventListener("change", (ev) => {
    const id = ev.target.dataset.stumble;
    if (!id) return;
    const l = loadLog();
    if (ev.target.checked) l.pasos.push(id);
    else l.pasos = l.pasos.filter((x) => x !== id);
    localStorage.setItem(STORE_KEY, JSON.stringify(l));
    registrar("friccion", ev.target.checked ? id : "deshecho");
  });
}

async function copiar(texto, donde) {
  try {
    await navigator.clipboard.writeText(texto);
    estadoZip.textContent = "copiado";
  } catch (e) {
    estadoZip.textContent = "no se pudo copiar: selecciona el texto de " + donde;
  }
}

document.getElementById("descargar").addEventListener("click", bajar);
document.getElementById("copiar-comandos").addEventListener("click",
  () => copiar(document.getElementById("comandos").textContent, "#comandos"));
document.getElementById("analizar").addEventListener("click", analizarPegado);
document.getElementById("ver-informe").addEventListener("click", () => {
  const out = document.getElementById("informe-out");
  out.hidden = false;
  out.textContent = informe();
  registrar("informe", null);
});
document.getElementById("borrar-informe").addEventListener("click", () => {
  localStorage.removeItem(STORE_KEY);
  document.getElementById("informe-out").textContent = "(borrado; recarga la página)";
  document.querySelectorAll("[data-stumble]").forEach((c) => { c.checked = false; });
});
form.addEventListener("input", escribir);
document.getElementById("n-reglas").textContent = RULES.length;
pintarFriccion();
escribir();
