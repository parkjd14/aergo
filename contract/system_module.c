#include <string.h>
#include <stdlib.h>
#include <time.h>
#include "vm.h"
#include "util.h"
#include "_cgo_export.h"

#define STATE_DB_KEY_PREFIX "_"

extern const int *getLuaExecContext(lua_State *L);

static int systemPrint(lua_State *L)
{
    char *jsonValue;
	int *service = (int *)getLuaExecContext(L);

    jsonValue = lua_util_get_json_from_stack (L, 1, lua_gettop(L), true);
    if (jsonValue == NULL) {
		lua_error(L);
	}
    LuaPrint(service, jsonValue);
    free(jsonValue);
	return 0;
}

static char *getDbKey(lua_State *L)
{
	lua_pushvalue(L, 1);    /* prefix key */
	lua_concat(L, 2);       /* dbKey(prefix..key) */
	return (char *)lua_tostring(L, -1);
}

int setItemWithPrefix(lua_State *L)
{
	char *dbKey;
	char *jsonValue;
	int *service = (int *)getLuaExecContext(L);

	if (service == NULL) {
		luaL_error(L, "cannot find execution context");
	}

	luaL_checkstring(L, 1);
	luaL_checkany(L, 2);
	luaL_checkstring(L, 3);
	dbKey = getDbKey(L);
	jsonValue = lua_util_get_json (L, 2, false);
	if (jsonValue == NULL) {
		lua_error(L);
	}

	if (LuaSetDB(L, service, dbKey, jsonValue) != 0) {
		free(jsonValue);
		lua_error(L);
	}
	free(jsonValue);

	return 0;
}

int setItem(lua_State *L)
{
	luaL_checkstring(L, 1);
	luaL_checkany(L, 2);
	lua_pushstring(L, STATE_DB_KEY_PREFIX);
	return setItemWithPrefix(L);
}

int getItemWithPrefix(lua_State *L)
{
	char *dbKey;
	int *service = (int *)getLuaExecContext(L);
	char *jsonValue;
	int ret;

	if (service == NULL) {
		luaL_error(L, "cannot find execution context");
	}
	luaL_checkstring(L, 1);
	luaL_checkstring(L, 2);
	dbKey = getDbKey(L);
	ret = LuaGetDB(L, service, dbKey);
	if (ret < 0) {
		lua_error(L);
	}
	if (ret == 0)
		return 0;
	jsonValue = (char *)luaL_checkstring(L, -1);
	lua_pop(L, 1);

	if (lua_util_json_to_lua(L, jsonValue, false) != 0) {
		luaL_error(L, "getItem error : can't convert %s", jsonValue);
	}
	return 1;
}

int getItem(lua_State *L)
{
	luaL_checkstring(L, 1);
	lua_pushstring(L, STATE_DB_KEY_PREFIX);
	return getItemWithPrefix(L);
}

int delItemWithPrefix(lua_State *L)
{
	char *dbKey;
	int *service = (int *)getLuaExecContext(L);
	char *jsonValue;
	int ret;

	if (service == NULL) {
		luaL_error(L, "cannot find execution context");
	}
	luaL_checkstring(L, 1);
	luaL_checkstring(L, 2);
	dbKey = getDbKey(L);
	ret = LuaDelDB(L, service, dbKey);
	if (ret < 0) {
		lua_error(L);
	}
    return 0;
}

static int getSender(lua_State *L)
{
	int *service = (int *)getLuaExecContext(L);
	if (service == NULL) {
		luaL_error(L, "cannot find execution context");
	}
	LuaGetSender(L, service);
	return 1;
}

static int getTxhash(lua_State *L)
{
	int *service = (int *)getLuaExecContext(L);
	if (service == NULL) {
		luaL_error(L, "cannot find execution context");
	}
	LuaGetHash(L, service);
	return 1;
}

static int getBlockHeight(lua_State *L)
{
	int *service = (int *)getLuaExecContext(L);
	if (service == NULL) {
		luaL_error(L, "cannot find execution context");
	}
	LuaGetBlockNo(L, service);
	return 1;
}

static int getTimestamp(lua_State *L)
{
	int *service = (int *)getLuaExecContext(L);
	if (service == NULL) {
		luaL_error(L, "cannot find execution context");
	}
	LuaGetTimeStamp(L, service);
	return 1;
}

static int getContractID(lua_State *L)
{
	int *service = (int *)getLuaExecContext(L);
	if (service == NULL) {
		luaL_error(L, "cannot find execution context");
	}
	LuaGetContractId(L, service);
	return 1;
}

static int getCreator(lua_State *L)
{
	int *service = (int *)getLuaExecContext(L);
	int ret;

	if (service == NULL) {
		luaL_error(L, "cannot find execution context");
	}
	ret = LuaGetDB(L, service, "Creator");
	if (ret < 0) {
		lua_error(L);
	}
	if (ret == 0)
		return 0;
	luaL_checkstring(L, -1);
	return 1;
}

static int getAmount(lua_State *L)
{
	int *service = (int *)getLuaExecContext(L);
	if (service == NULL) {
		luaL_error(L, "cannot find execution context");
	}
	LuaGetAmount(L, service);
	return 1;
}

static int getOrigin(lua_State *L)
{
	int *service = (int *)getLuaExecContext(L);
	if (service == NULL) {
		luaL_error(L, "cannot find execution context");
	}
	LuaGetOrigin(L, service);
	return 1;
}

/* datetime-related functions from lib_os.c. time(NULL) is replaced by blocktime(L) */

static void setfield(lua_State *L, const char *key, int value)
{
    lua_pushinteger(L, value);
    lua_setfield(L, -2, key);
}

static void setboolfield(lua_State *L, const char *key, int value)
{
    if (value < 0)  /* undefined? */
        return;  /* does not set field */
    lua_pushboolean(L, value);
    lua_setfield(L, -2, key);
}

static int getboolfield(lua_State *L, const char *key)
{
    int res;
    lua_getfield(L, -1, key);
    res = lua_isnil(L, -1) ? -1 : lua_toboolean(L, -1);
    lua_pop(L, 1);
    return res;
}

static int getfield(lua_State *L, const char *key, int d)
{
    int res;
    lua_getfield(L, -1, key);
    if (lua_isnumber(L, -1)) {
        res = (int)lua_tointeger(L, -1);
    } else {
        if (d < 0)
            luaL_error(L, "field " LUA_QS " missing in date table", key);
        res = d;
    }
    lua_pop(L, 1);
    return res;
}

static time_t blocktime(lua_State *L)
{
    time_t t;
    getTimestamp(L);
    t = (time_t)lua_tointeger(L, -1);
    lua_pop(L, 1);
    return t;
}

static int os_date(lua_State *L)
{
    const char *s = luaL_optstring(L, 1, "%c");
    time_t t = luaL_opt(L, (time_t)luaL_checknumber, 2, blocktime(L));
    struct tm *stm;
#if LJ_TARGET_POSIX
    struct tm rtm;
#endif
    if (*s == '!') {  /* UTC? */
        s++;  /* Skip '!' */
    }
#if LJ_TARGET_POSIX
    stm = gmtime_r(&t, &rtm);
#else
    stm = gmtime(&t);
#endif
    if (stm == NULL) {  /* Invalid date? */
        lua_pushnil(L);
    } else if (strcmp(s, "*t") == 0) {
        lua_createtable(L, 0, 9);  /* 9 = number of fields */
        setfield(L, "sec", stm->tm_sec);
        setfield(L, "min", stm->tm_min);
        setfield(L, "hour", stm->tm_hour);
        setfield(L, "day", stm->tm_mday);
        setfield(L, "month", stm->tm_mon+1);
        setfield(L, "year", stm->tm_year+1900);
        setfield(L, "wday", stm->tm_wday+1);
        setfield(L, "yday", stm->tm_yday+1);
        setboolfield(L, "isdst", stm->tm_isdst);
    } else {
        char cc[3];
        luaL_Buffer b;
        cc[0] = '%'; cc[2] = '\0';
        luaL_buffinit(L, &b);
        for (; *s; s++) {
            if (*s != '%' || *(s + 1) == '\0') {  /* No conversion specifier? */
                luaL_addchar(&b, *s);
            } else {
                size_t reslen;
                char buff[200];  /* Should be big enough for any conversion result. */
                cc[1] = *(++s);
                if (cc[1] == 'c') {
                    reslen = strftime(buff, sizeof(buff), "%x", stm);
                    luaL_addlstring(&b, buff, reslen);
                    luaL_addchar(&b, ' ');
                    reslen = strftime(buff, sizeof(buff), "%X", stm);
                    luaL_addlstring(&b, buff, reslen);
                } else {
                    reslen = strftime(buff, sizeof(buff), cc, stm);
                    luaL_addlstring(&b, buff, reslen);
                }
            }
        }
        luaL_pushresult(&b);
    }
    return 1;
}

static int os_time(lua_State *L)
{
    time_t t;
    if (lua_isnoneornil(L, 1)) {
        t = blocktime(L);
    } else {
        struct tm ts;
        luaL_checktype(L, 1, LUA_TTABLE);
        lua_settop(L, 1);  /* make sure table is at the top */
        ts.tm_sec = getfield(L, "sec", 0);
        ts.tm_min = getfield(L, "min", 0);
        ts.tm_hour = getfield(L, "hour", 12);
        ts.tm_mday = getfield(L, "day", -1);
        ts.tm_mon = getfield(L, "month", -1) - 1;
        ts.tm_year = getfield(L, "year", -1) - 1900;
        ts.tm_isdst = getboolfield(L, "isdst");
#if LJ_TARGET_POSIX
        t = timegm(&ts);
#else
        t = _mkgmtime(&ts);
#endif
    }
    if (t == (time_t)(-1))
        lua_pushnil(L);
    else
        lua_pushnumber(L, (lua_Number)t);
    return 1;
}

static int os_difftime(lua_State *L)
{
    lua_pushnumber(L, difftime((time_t)(luaL_checknumber(L, 1)),
                (time_t)(luaL_optnumber(L, 2, (lua_Number)0))));
    return 1;
}

/* end of datetime functions */

static const luaL_Reg sys_lib[] = {
	{"print", systemPrint},
	{"setItem", setItem},
	{"getItem", getItem},
	{"getSender", getSender},
	{"getCreator", getCreator},
	{"getTxhash", getTxhash},
	{"getBlockheight", getBlockHeight},
	{"getTimestamp", getTimestamp},
	{"getContractID", getContractID},
	{"getOrigin", getOrigin},
	{"getAmount", getAmount},
	{"date", os_date},
	{"time", os_time},
	{"difftime", os_difftime},
	{NULL, NULL}
};

int luaopen_system(lua_State *L)
{
	luaL_register(L, "system", sys_lib);
	lua_pop(L, 1);
	return 1;
}
