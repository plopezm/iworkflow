// Package errors holds the business errors of the agent_profiles module.
//
// The messages are the Spanish texts of the Rust version and are kept byte-for-byte: parity
// requires the same text, and the Rust app never translated them.
package errors

// Error is a business failure. Two errors match with errors.Is when they have the same kind, so
// callers can test `errors.Is(err, errors.ProfileNotFound(""))` without knowing the detail.
type Error struct {
	kind   kind
	detail string
}

type kind uint8

const (
	kindInvalidProfileName kind = iota + 1
	kindInvalidRoleName
	kindProfileNotFound
	kindProfileAlreadyExists
	kindCannotDeleteActiveProfile
	kindInvalidRouting
	kindInvalidImport
	kindRoleAlreadyExists
	kindIO
	kindStore
)

// Error returns the message shown to the user.
func (e Error) Error() string {
	switch e.kind {
	case kindInvalidProfileName:
		return "nombre de perfil inválido: " + e.detail
	case kindInvalidRoleName:
		return "nombre de rol inválido: " + e.detail
	case kindProfileNotFound:
		return "perfil no encontrado: " + e.detail
	case kindProfileAlreadyExists:
		return "el perfil ya existe: " + e.detail
	case kindCannotDeleteActiveProfile:
		return "no se puede eliminar el perfil activo: " + e.detail
	case kindInvalidRouting:
		return "routing inválido: " + e.detail
	case kindInvalidImport:
		return "importación inválida: " + e.detail
	case kindRoleAlreadyExists:
		return "el rol ya existe: " + e.detail
	case kindIO:
		return "error de E/S: " + e.detail
	case kindStore:
		return "error de almacén de perfiles: " + e.detail
	default:
		return "error desconocido: " + e.detail
	}
}

// Is matches any error of the same kind, regardless of its detail.
func (e Error) Is(target error) bool {
	other, ok := target.(Error)
	return ok && other.kind == e.kind
}

// InvalidProfileName means the raw text does not match the profile name format.
func InvalidProfileName(raw string) Error { return Error{kind: kindInvalidProfileName, detail: raw} }

// InvalidRoleName means the raw text does not match the role name format.
func InvalidRoleName(raw string) Error { return Error{kind: kindInvalidRoleName, detail: raw} }

// ProfileNotFound means no profile has the given name.
func ProfileNotFound(name string) Error { return Error{kind: kindProfileNotFound, detail: name} }

// ProfileAlreadyExists means a profile with the given name is already in the store.
func ProfileAlreadyExists(name string) Error {
	return Error{kind: kindProfileAlreadyExists, detail: name}
}

// CannotDeleteActiveProfile means the caller tried to delete the profile marked as active.
func CannotDeleteActiveProfile(name string) Error {
	return Error{kind: kindCannotDeleteActiveProfile, detail: name}
}

// InvalidRouting means a model, a thinking level or an agent key is not acceptable.
func InvalidRouting(detail string) Error { return Error{kind: kindInvalidRouting, detail: detail} }

// InvalidImport means an exchange document cannot be read as a profile.
func InvalidImport(detail string) Error { return Error{kind: kindInvalidImport, detail: detail} }

// RoleAlreadyExists means an agent role with the given name already exists.
func RoleAlreadyExists(name string) Error { return Error{kind: kindRoleAlreadyExists, detail: name} }

// IO wraps a failure of the file system.
func IO(detail string) Error { return Error{kind: kindIO, detail: detail} }

// Store wraps a failure of a configuration file (profiles.json, models.json, the exchange store).
func Store(detail string) Error { return Error{kind: kindStore, detail: detail} }
