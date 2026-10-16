use crate::core::process::kill_tree;

/// Mata la shell `root` y todo lo que lanzó (`taskkill /T`). A diferencia de Unix no hay sesiones
/// que barrer: Windows solo conserva el árbol de padres. La shell también muere aquí; el
/// llamador la mata de nuevo sin efecto.
pub fn kill_descendants(root: u32) {
    kill_tree(root);
}
