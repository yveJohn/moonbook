const DECIMAL_LONG_ID = /^[1-9][0-9]*$/

export const assertLongId = (value) => {
  if (typeof value !== 'string' || !DECIMAL_LONG_ID.test(value)) {
    throw new TypeError('Long ID must be a positive decimal string')
  }
  return value
}

export const appendLongId = (path, value) =>
  `${path}/${encodeURIComponent(assertLongId(value))}`
