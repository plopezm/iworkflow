/// Modelo del orquestador seleccionable vía `--provider`/`--model` de pi.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ModelRef {
    pub provider: String,
    pub model: String,
}
