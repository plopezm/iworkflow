# windows-native
Soporte nativo de Windows para el servidor iworkflow (antes solo Linux/macOS).
Rama: pi/compatibilidad-con-windows-*. No se puede ejecutar Windows aquí: verificación por lógica pura testeada en Linux + compilación cruzada si es posible.

## Specs
- S1: "Vamos a hacerlo nativo, para el 3 usa powershell" (L2) — soporte Windows nativo; la shell por defecto en Windows es PowerShell.
- S2: `nix` solo se compila en unix (dependencia por target); matar el árbol de procesos de la terminal debe funcionar en Windows.
- S3: Shell/entorno: en Windows la shell es `powershell` y se lanza sin `-l`/`exec`; el entorno mínimo usa variables de Windows (USERPROFILE, SystemRoot, PATH heredado...). El comportamiento Unix no cambia.
- S4: El home se resuelve con `HOME` y, si falta, `USERPROFILE`.
- S5: Los agentes lanzados vía PowerShell se matan como árbol en Windows (no hay `exec`).
- S6: Tests Unix-only protegidos con `#[cfg(unix)]`; los tests de rutas no asumen `/`. Documentar el soporte.

## Tasks
- T1 (S2) Cargo `nix` por target + process_tree Windows — commit
- T2 (S1,S3) `ShellFlavor` en core/env.rs y callers (pty, agent_process, catálogo) — commit
- T3 (S4) Home con fallback USERPROFILE — commit
- T4 (S5) Kill de árbol de agente en Windows — commit
- T5 (S6) Tests portables, docs, fmt/clippy/tests — commit

## Log
- L1: Es compatible con windows?
- L2: Vamos a hacerlo nativo, para el 3 usa powershell

## Evidence
- T1, T2, T4: commits 678accb, 1b6e1d6 (T2 y T4 comparten fichero, un solo commit).
- T3, T5: commit final de la rama (config.rs, README, este documento).
- Verificado en Linux: `cargo fmt --check`, clippy ssr y hydrate `-D warnings`, `cargo test --features ssr --lib` (1067 ok).
- No verificado: compilación/ejecución en Windows (sin mingw local; lo cubre el workflow release.yml con windows-latest).
