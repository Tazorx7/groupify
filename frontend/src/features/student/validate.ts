// Client-side mirrors of the backend's student-input rules
// (backend/internal/domain/types.go). The server remains the authority; this
// only exists so the obvious mistakes are caught before a round trip.

export const MAX_NAME_LENGTH = 120
export const MAX_INDEX_LENGTH = 40

const INDEX_RUNES = /^[A-Z0-9 /._-]+$/

/** Trim, collapse internal whitespace, upper-case — same as the server. */
export function normalizeIndexNumber(raw: string): string {
  return raw.replace(/\s+/g, ' ').trim().toUpperCase()
}

export function normalizeName(raw: string): string {
  return raw.replace(/\s+/g, ' ').trim()
}

export interface FieldErrors {
  full_name?: string
  index_number?: string
}

export function validateStudent(fullName: string, indexNumber: string): FieldErrors {
  const name = normalizeName(fullName)
  const index = normalizeIndexNumber(indexNumber)
  const errors: FieldErrors = {}

  if (!name) {
    errors.full_name = 'Enter your full name.'
  } else if ([...name].length > MAX_NAME_LENGTH) {
    errors.full_name = `Full name is too long (max ${MAX_NAME_LENGTH} characters).`
  } else if (!/\p{L}/u.test(name)) {
    errors.full_name = 'Full name must contain letters.'
  }

  if (!index) {
    errors.index_number = 'Enter your index number.'
  } else if ([...index].length > MAX_INDEX_LENGTH) {
    errors.index_number = `Index number is too long (max ${MAX_INDEX_LENGTH} characters).`
  } else if (!/[A-Z0-9]/.test(index)) {
    errors.index_number = 'Index number must contain at least one letter or digit.'
  } else if (!INDEX_RUNES.test(index)) {
    errors.index_number = 'Use only letters, digits, spaces and / - . _ characters.'
  }

  return errors
}

export function hasErrors(errors: FieldErrors): boolean {
  return Object.keys(errors).length > 0
}
