package main

// La sesión viva de `apkc preview --sesion <apk>`. El hospedador mantiene UNA instancia wasm y
// manda solo la orden nueva; aquí la máquina y su resultado siguen en memoria entre órdenes, así
// que un clic anterior no se repite cuando llega el hot swap. Nada de esta lógica llama a os.Exit:
// el wrapper del comando es el único que sale, como en wisp_get.go:66-68,93-98.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// sesionPreview es el estado que sobrevive a todas las órdenes de una instancia.
type sesionPreview struct {
	datos   []byte
	layout  string
	locale  string
	paquete string
	res     *resultadoEjecucion
	m       *maquinaDalvik
	// pila guarda las actividades cubiertas por un `startActivity`, en orden, para que un
	// `finish()` pueda volver a la que quedó detrás conservando su máquina y su estado.
	pila []*maquinaDalvik
	// pantallas es el recorrido acumulado de actividades, con la actual al final. actualizaRes lo
	// publica en res.Pantallas; se guarda aparte para no perderlo cuando cambia la máquina delantera.
	pantallas []string
	// base es el dex contra el que se calcula el SIGUIENTE delta. Solo se cambia cuando aplicaDelta
	// terminó bien; en un fallo la sesión sigue interpretando el dex anterior.
	base *dexFile
	// salir y publica se inyectan: el wrapper pone os.Exit y el publicador del wasm; las pruebas
	// ponen contadores y comprueban que salir nunca se llama por una orden.
	salir   func(int)
	publica func(id any, codigo int)
}

// Contrato del hot swap de recursos, paralelo al del dex:
//
//   - `--intercambiar firma=ruta.dex` sustituye el código vivo (deltaDeDexes + aplicaDelta);
//   - `--intercambiar-recursos ruta.apk` sustituye la fuente de la interfaz. El apk nuevo viaja
//     como ruta porque en la pestaña el hospedador lo escribe en el fs de OPFS antes de la orden,
//     igual que hace con el dex; en nativo es un fichero cualquiera.
//
// Los dos flags pueden ir en la MISMA orden y conservan el id de sesión y el estado de la máquina:
// ninguno reinicia onCreate ni recoloca los objetos vivos. El evento de recursos se encola al final
// del guion para que, si el dex o cualquier otra orden falla, la sesión siga leyendo el apk viejo.
// Los bytes del apk viajan en `eventoUsuario.Dex` (el campo ya existe para el dex y aquí se reusa;
// no se añade un campo nuevo al tipo compartido con el comando de una pasada).
const (
	banderaIntercambiarRecursos   = "intercambiar-recursos"
	ordenIntercambiarRecursosTipo = "intercambiar-recursos"
)

// extraeIntercambiarRecursos saca del argv de la orden `--intercambiar-recursos <ruta.apk>` (o su
// forma `--intercambiar-recursos=<ruta.apk>`), lee el apk y devuelve el argv sin esa bandera. Se hace
// ANTES de parsePreview: la bandera es de la sesión viva, no del comando de una pasada, así que no se
// registra en registrarBanderasPreview (preview.go) y el parser de flag común nunca la ve.
func extraeIntercambiarRecursos(argv []string) ([]string, []byte, error) {
	limpio := make([]string, 0, len(argv))
	var datos []byte
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		valor, conIgual := strings.CutPrefix(arg, "--"+banderaIntercambiarRecursos+"=")
		if !conIgual && arg == "--"+banderaIntercambiarRecursos {
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--%s necesita la ruta del apk nuevo", banderaIntercambiarRecursos)
			}
			i++
			valor = argv[i]
		} else if !conIgual {
			limpio = append(limpio, arg)
			continue
		}
		if strings.TrimSpace(valor) == "" {
			return nil, nil, fmt.Errorf("--%s necesita la ruta del apk nuevo", banderaIntercambiarRecursos)
		}
		if datos != nil {
			return nil, nil, fmt.Errorf("--%s no se puede repetir en una misma orden", banderaIntercambiarRecursos)
		}
		leidos, err := os.ReadFile(valor)
		if err != nil {
			return nil, nil, fmt.Errorf("--%s: %w", banderaIntercambiarRecursos, err)
		}
		datos = leidos
	}
	return limpio, datos, nil
}

// modeloDeBytes es el arranque de la sesión: el mismo modelo que el comando normal, sin leer un
// fichero porque la apk ya está en memoria.
func modeloDeBytes(datos []byte, layout, locale string, guion []eventoUsuario) (*modeloVista, error) {
	return modeloDeBytesConOpciones(datos, opcionesPreview{layout: layout, locale: locale, guion: guion})
}

func modeloDeBytesConOpciones(datos []byte, o opcionesPreview) (*modeloVista, error) {
	return modeloVistaDeAPKConEventos(datos, o.layout, !o.sinEjecutar, o.guion, o.locale)
}

// nuevaSesionPreview abre la apk una vez y deja la máquina viva en `onCreate`, sin órdenes.
func nuevaSesionPreview(datos []byte, layout, locale string) (*sesionPreview, error) {
	modelo, err := modeloDeBytes(datos, layout, locale, nil)
	if err != nil {
		return nil, err
	}
	if modelo.Ejecucion == nil || modelo.Ejecucion.maquina == nil {
		return nil, fmt.Errorf("la sesión necesita un classes.dex ejecutable")
	}
	m := modelo.Ejecucion.maquina
	return &sesionPreview{
		datos: datos, layout: layout, locale: locale, paquete: modelo.Paquete,
		res: modelo.Ejecucion, m: m, base: m.prog.dex,
		pantallas: append([]string(nil), modelo.Ejecucion.Pantallas...),
	}, nil
}

// preparadorSesion enlaza los `android:onClick` del layout de una actividad recién abierta, igual
// que el comando de una sola pasada hace en preview.go:751. La sesión no guarda el zip ni la tabla
// del arranque, así que los reconstruye de los bytes de la apk que ya tiene en memoria.
func (s *sesionPreview) preparadorSesion() func(*maquinaDalvik) {
	return func(m *maquinaDalvik) {
		zr, err := zip.NewReader(bytes.NewReader(s.datos), int64(len(s.datos)))
		if err != nil {
			return
		}
		var tabla *tablaRecursos
		if arsc, err := entradaDeZip(zr, "resources.arsc"); err == nil {
			tabla, err = leeTabla(arsc)
			if err == nil && s.locale != "" {
				tabla.locale = s.locale
			}
		}
		preparaVistasDelLayout(m, zr, tabla, s.layout)
	}
}

// estabilizaSesion hace por la sesión viva lo que el cierre `estabiliza` de previewinterprete.go:1453
// hace en una pasada: abre la actividad que dejó pendiente un `startActivity` y vuelve a la anterior
// si alguien llamó a `finish()`. La máquina que queda delante y su pila se guardan en la sesión para
// que la siguiente orden llegue a la pantalla correcta, sin releer el dex ni reenviar el guion.
func (s *sesionPreview) estabilizaSesion() {
	for {
		m := s.m
		if m.pendiente != "" {
			destino := m.pendiente
			m.pendiente = ""
			if len(s.pantallas) >= pantallasMaximas {
				// el tope simétrico de los clics: corta una navegación circular antes de encadenar
				// pantallas sin fin, como en el comando de una pasada.
				m.avisa("startActivity a %s no se abrió: el tope de %d pantallas", claseCorta(destino),
					pantallasMaximas)
				continue
			}
			if _, ok := m.prog.clases[destino]; !ok {
				m.avisa("startActivity a %s: la clase no está en el dex", claseCorta(destino))
				continue
			}
			if m.declaradas != nil && !m.declaradas[destino] {
				m.avisa("startActivity a %s: no está declarada en el manifiesto", claseCorta(destino))
				continue
			}
			m.pausa()
			nueva := nuevaMaquina(m.prog, m.tabla)
			nueva.declaradas = m.declaradas
			nueva.registro = m.registro
			nueva.almacen = m.almacen
			nueva.reloj = m.reloj
			nueva.intent = m.intentoSaliente
			if err := nueva.abreActividad(destino, s.preparadorSesion(), true); err != nil {
				m.avisa("la actividad %s no arrancó: %v", claseCorta(destino), err)
				m.reanuda()
				continue
			}
			s.pila = append(s.pila, m)
			s.m = nueva
			s.pantallas = append(s.pantallas, claseCorta(destino))
			continue
		}
		if m.terminada {
			m.terminada = false
			m.cierra()
			if len(s.pila) > 0 {
				anterior := s.pila[len(s.pila)-1]
				s.pila = s.pila[:len(s.pila)-1]
				s.m = anterior
				anterior.reanuda()
				s.pantallas = append(s.pantallas, claseCorta(anterior.actividad.clase))
				continue
			}
			m.avisa("finish() en la única actividad: la app se queda sin pantalla")
		}
		return
	}
}

// aplicaEvento entrega UNA orden a la máquina viva. El intercambio pasa siempre por deltaDeDexes
// y aplicaDelta; el dex base solo se sustituye después de un éxito.
func (s *sesionPreview) aplicaEvento(ev eventoUsuario) error {
	if ev.Tipo == ordenIntercambiarRecursosTipo {
		// Validar antes de mutar: un zip válido sin el layout de esta actividad también es un apk
		// irrepresentable, y basta con mirar el zip para no verlo. Se construye el modelo contra
		// una copia de la sesión con los bytes candidatos; solo si se resuelve y lee el AXML se
		// sustituye `s.datos`. No hay caché que invalidar: modeloConResultado abre `s.datos` y
		// re-parsea resources.arsc y el layout en CADA orden. La máquina (y su contador) no se toca.
		candidata := *s
		candidata.datos = ev.Dex
		if _, err := candidata.modeloConResultado(); err != nil {
			return fmt.Errorf("--%s: el apk nuevo no sirve para esta sesión: %w",
				banderaIntercambiarRecursos, err)
		}
		s.datos = ev.Dex
		return nil
	}
	if ev.Tipo == ordenIntercambiarTipo {
		nuevo, err := readDex(ev.Dex)
		if err != nil {
			return fmt.Errorf("--intercambiar %s: %w", ev.Dato, err)
		}
		cambios, err := deltaDeDexes(s.base, nuevo)
		if err != nil {
			return fmt.Errorf("--intercambiar %s: %w", ev.Dato, err)
		}
		hechos, err := s.m.aplicaDelta(cambios, nuevo)
		if err != nil {
			return fmt.Errorf("--intercambiar %s: %w", ev.Dato, err)
		}
		s.base = nuevo
		s.res.Intercambios = append(s.res.Intercambios, hechos...)
		return nil
	}
	if ev.Tipo == ordenAvanzarTipo {
		// La sesión viva tiene que mover el reloj igual que la pasada única (previewinterprete.go:
		// 1593-1605): `entregaEvento` no tiene caso para esta orden y antes se descartaba sin tocar
		// el reloj, así que el cronómetro nunca avanzaba aunque el fin-orden saliera con código 0.
		ms, err := strconv.ParseInt(ev.Dato, 10, 64)
		if err != nil || ms < 0 {
			return fmt.Errorf("--avanzar %q no son milisegundos enteros no negativos", ev.Dato)
		}
		if err := s.m.avanza(ms); err != nil {
			return err
		}
		// El mismo drenaje de `drena` en la pasada única (previewinterprete.go:1556-1562): correr
		// las tareas que vencen hasta el instante pedido, estabilizando la navegación entre ellas,
		// y dejar el reloj en el límite aunque una tarea lo haya movido a su propio instante.
		limite := s.m.ahora()
		for s.m.vence(limite) {
			s.estabilizaSesion()
		}
		if !s.m.cortada && s.m.ahora() < limite && s.m.reloj != nil {
			*s.m.reloj = limite
		}
		return nil
	}
	if err := s.m.entregaEvento(s.m.tabla, ev, s.res); err != nil {
		return err
	}
	// un clic puede llamar a `startActivity` o a `finish()`: estabilizar aquí abre la actividad
	// siguiente (o vuelve a la anterior) antes de publicar, igual que hace el bucle de una pasada
	// entre orden y orden en previewinterprete.go:1648. Sin esto la máquina viva seguía delante de
	// la pantalla vieja aunque el `onClick` hubiera corrido.
	s.estabilizaSesion()
	// las tareas vencidas pueden navegar a otra pantalla: estabiliza también tras cada una, como
	// hace `drena` en el comando de una sola pasada (previewinterprete.go:1555-1558).
	for s.m.vence(s.m.ahora()) {
		s.estabilizaSesion()
	}
	return nil
}

// actualizaRes vuelca la máquina viva sobre el resultado SIN sumar dos veces: los contadores se
// asignan, no se incrementan, porque la misma pantalla se publica tras cada orden.
func (s *sesionPreview) actualizaRes() {
	m, res := s.m, s.res
	// un error es de LA orden que lo produjo, no del modelo acumulado: si una orden mala no lo
	// limpiara, la siguiente buena seguiría publicando `error` con codigo 0 (json:"error,omitempty")
	res.Error = ""
	res.Actividad = claseCorta(m.actividad.clase)
	// el recorrido se publica desde la sesión, no desde la máquina delantera: `startActivity` y
	// `finish()` cambian la máquina, y el historial de pantallas tiene que seguir acumulado para
	// que el panel y la vista Pantalla vean la actividad que quedó delante.
	res.Pantallas = append([]string(nil), s.pantallas...)
	res.Capturas = append([]capturaTexto(nil), m.capturas...)
	res.Oyentes = m.oyentesDeVista()
	res.Marcas = m.marcasDeVista()
	res.Pistas = m.pistasDeVista()
	res.Pasos = m.pasos
	res.Interacciones = m.interaccion
	res.Tareas = m.tareas
	res.Consola = append([]lineaConsola(nil), m.consola...)
	for i := range res.Consola {
		res.Consola[i].Secuencia = i + 1
	}
	res.Avisos = nil
	for _, a := range m.avisos {
		if !contieneTexto(res.Avisos, a) {
			res.Avisos = append(res.Avisos, a)
		}
	}
	res.Reloj = m.ahora()
	if m.registro != nil {
		res.Ciclo = append([]string(nil), *m.registro...)
	}
	res.Dialogos = m.dialogosDeVista()
	res.Menu = m.menuVista()
	res.Toast = m.toast
	res.Preferencias = m.almacen.volcado()
	if m.estadoGuardado != nil {
		res.Bundle = m.estadoGuardado
	}
	res.maquina = m
}

// modeloConResultado vuelve a pintar el layout con el resultado acumulado; es la cola de
// modeloVistaDeAPKConEventos (preview.go:715-754) con la apk ya abierta. No guarda ni el zip, ni la
// tabla, ni el AXML: los abre desde `s.datos` en CADA orden. Por eso el hot swap de recursos solo
// tiene que sustituir `s.datos` para que el siguiente modelo traiga el main.xml nuevo, sin tocar la
// máquina ni su estado.
func (s *sesionPreview) modeloConResultado() (*modeloVista, error) {
	zr, err := zip.NewReader(bytes.NewReader(s.datos), int64(len(s.datos)))
	if err != nil {
		return nil, fmt.Errorf("el apk no es un zip: %w", err)
	}
	var tabla *tablaRecursos
	if arsc, err := entradaDeZip(zr, "resources.arsc"); err == nil {
		if tabla, err = leeTabla(arsc); err != nil {
			return nil, err
		}
		if s.locale != "" {
			tabla.locale = s.locale
		}
	}
	ruta, err := rutaDelLayoutDeLaActividad(tabla, zr, s.layout, s.m.layoutID)
	if err != nil {
		return nil, err
	}
	layoutBytes, err := entradaDeZip(zr, ruta)
	if err != nil {
		return nil, err
	}
	axml, err := leeAXML(layoutBytes)
	if err != nil {
		return nil, err
	}
	ctx := &contextoVista{axml: axml, tabla: tabla}
	arbol, err := ctx.arbol()
	if err != nil {
		return nil, err
	}
	raiz := ctx.nodoDe(arbol)
	if raiz == nil {
		return nil, fmt.Errorf("el layout %s no tiene raíz", ruta)
	}
	cargaImagenes(raiz, zr, tabla)
	coloca(raiz, 0, 0, dpPantallaAncho, dpPantallaAlto)
	anotaTextoDinamico(raiz, s.res)
	anotaOyentes(raiz, s.res)
	anotaEstados(raiz, s.res)
	pintaVistasPropias(raiz, s.res, s.paquete)
	expandeListas(raiz, s.res, tabla, zr)
	coloca(raiz, 0, 0, dpPantallaAncho, dpPantallaAlto)
	modelo := &modeloVista{Aproximado: true, Paquete: s.paquete,
		Pantalla:  rectVista{Ancho: dpPantallaAncho, Alto: dpPantallaAlto},
		Actividad: s.res.Actividad, Layout: ruta, Raiz: raiz, Ejecucion: s.res,
		Nodos: contarNodos(raiz)}
	if len(s.res.Dialogos) > 0 {
		capa := s.res.Dialogos[len(s.res.Dialogos)-1]
		modelo.Dialogo = &capa
	}
	return modelo, nil
}

// ejecutaOrden es puro: parsea SOLO el argv de la orden, la aplica, escribe UNA línea JSON y
// publica el fin de orden una sola vez, después de la línea para que consola.volcar() la vea.
// Nunca llama a salir: el wrapper es el único que puede hacerlo.
func (s *sesionPreview) ejecutaOrden(id any, argv []string, escribir func(string)) int {
	publica := func(codigo int) {
		if s.publica != nil {
			s.publica(id, codigo)
		}
	}
	falla := func(motivo string) int {
		datos, _ := json.Marshal(map[string]any{"error": motivo})
		escribir(string(datos))
		publica(2)
		return 2
	}
	argvLimpio, recursos, errRecursos := extraeIntercambiarRecursos(argv)
	o, posicionales, err := parsePreview(argvLimpio)
	if err == nil && errRecursos != nil {
		err = errRecursos
	}
	if err == nil && len(posicionales) > 0 {
		// el apk va en el argv de ARRANQUE; una orden no puede traer otro
		err = fmt.Errorf("la orden no acepta un apk: %s", posicionales[0])
	}
	if err == nil && recursos != nil {
		// al final del guion: si el dex o cualquier orden anterior falla, no se sustituye el apk
		o.guion = append(o.guion, eventoUsuario{Tipo: ordenIntercambiarRecursosTipo, Dex: recursos})
	}
	if err != nil {
		s.actualizaRes()
		s.res.Error = err.Error()
		modelo, merr := s.modeloConResultado()
		if merr != nil {
			return falla(err.Error())
		}
		datos, merr := json.Marshal(modelo)
		if merr != nil {
			return falla(err.Error())
		}
		escribir(string(datos))
		publica(2)
		return 2
	}
	var fallo error
	for _, ev := range o.guion {
		if err := s.aplicaEvento(ev); err != nil {
			fallo = err
			break
		}
	}
	s.actualizaRes()
	if fallo != nil {
		s.res.Error = fallo.Error()
	}
	modelo, err := s.modeloConResultado()
	if err != nil {
		return falla(err.Error())
	}
	datos, err := json.Marshal(modelo)
	if err != nil {
		return falla(err.Error())
	}
	escribir(string(datos))
	if fallo != nil {
		publica(2)
		return 2
	}
	publica(0)
	return 0
}
