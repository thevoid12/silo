export const IPC = {
  GET_CONNECTION: 'silo:get-connection',
  VAULT_EXISTS: 'silo:vault-exists',
  SETUP: 'silo:setup',
  UNLOCK: 'silo:unlock',
  VAULT_LIST: 'silo:vault-list',
  VAULT_GET_ALL: 'silo:vault-get-all',
  VAULT_SET: 'silo:vault-set',
  VAULT_DELETE: 'silo:vault-delete',
  RESTART: 'silo:restart',
  READY: 'silo:ready',
  ERROR: 'silo:error',
} as const
