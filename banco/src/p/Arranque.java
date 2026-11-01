package p;

// El banco se ejecuta con `app_process` en el aparato, que exige un `main(String[])`. Este fork
// todavia no emite tipos de array (parrafo 5, item 5.11 del plan), asi que esta clase la compila
// javac y la nuestra (`p.Ver`) la escribe `apkc compile`: se dexan las dos con `apkc dex` y el
// resultado se compara con el de la JVM sobre las mismas fuentes.
public class Arranque {
    public static void main(String[] args) {
        Ver.cuenta();
        Ver.cuenta();
        int t = Ver.total;
        System.exit(t);
    }
}
