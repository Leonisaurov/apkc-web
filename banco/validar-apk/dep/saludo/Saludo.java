package saludo;

// La dependencia del fixture: vive en jar/saludo.jar, no en src/, y la app la llama. Es la misma
// forma que usa banco/deps-ejemplo, que ya dexa en verde con --compiler go: aquí se reutiliza para
// validar el empaquetado, la firma y los oráculos sin esperar al jar real.
public class Saludo {
    public static String saludo(String nombre) {
        return "hola, " + nombre;
    }
}
