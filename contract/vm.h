#ifndef _VM_H
#define _VM_H

#include <lualib.h>
#include <lauxlib.h>
#include <luajit.h>
#include "sqlite3-binding.h"

typedef struct blockchain_ctx {
	char *stateKey;
	char *sender;
	char *origin;
	char *contractId;
	char *txHash;
	unsigned long long blockHeight;
	long long timestamp;
	char *node;
	int confirmed;
	int isQuery;
	int transferFailed;
	int dbSystemError;
	unsigned long long rp;
	int service;
	char *amount;
} bc_ctx_t;

lua_State *vm_newstate();
int vm_isnil(lua_State *L, int idx);
void vm_getfield(lua_State *L, const char *name);
void vm_remove_construct(lua_State *L, const char *constructName);
const char *vm_loadbuff(lua_State *L, const char *code, size_t sz, int *service);
const char *vm_pcall(lua_State *L, int argc, int* nresult);
const char *vm_get_json_ret(lua_State *L, int nresult);
const char *vm_tostring(lua_State *L, int idx);
const char *vm_copy_result(lua_State *L, lua_State *target, int cnt);
void bc_ctx_delete(bc_ctx_t *bcctx);
sqlite3 *vm_get_db(lua_State *L);
void vm_get_abi_function(lua_State *L, char *fname);

#endif /* _VM_H */
