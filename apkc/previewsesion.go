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
)

// sesionPreview es el estado que sobrevive a todas las órdenes de una instancia.
type sesionPreview struct {
	datos   []byte
	layout  string
	locale  string
	paquete string
	res     *resultadoEjecucion
	m       *maquinaDalvik
	// base es el dex contra el que se calcula el SIGUIENTE delta. Solo se cambia cuando aplicaDelta
	// terminó bien; en un fallo la sesión sigue interpretando el dex anterior.
	base *dexFile
	// salir y publica se inyectan: el wrapper pone os.Exit y el publicador del wasm; las pruebas
	// ponen contadores y comprueban que salir nunca se llama por una orden.
	salir   func(int)
	publica func(id any, codigo int)
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
	}, nil
}

// aplicaEvento entrega UNA orden a la máquina viva. El intercambio pasa siempre por deltaDeDexes
// y aplicaDelta; el dex base solo se sustituye después de un éxito.
func (s *sesionPreview) aplicaEvento(ev eventoUsuario) error {
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
	if err := s.m.entregaEvento(s.m.tabla, ev, s.res); err != nil {
		return err
	}
	// el guion del hospedador no trae navegación; drenar las tareas vencidas basta para que el
	// reloj lógico y los aftermath de un clic queden como en el comando de una sola pasada.
	for s.m.vence(s.m.ahora()) {
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
// modeloVistaDeAPKConEventos (preview.go:715-754) con la apk ya abierta.
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
	o, posicionales, err := parsePreview(argv)
	if err == nil && len(posicionales) > 0 {
		// el apk va en el argv de ARRANQUE; una orden no puede traer otro
		err = fmt.Errorf("la orden no acepta un apk: %s", posicionales[0])
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
