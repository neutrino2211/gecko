#include <sqlite3.h>
#include <string.h>

int gecko_sqlite3_bind_double_bits(sqlite3_stmt *stmt, int index, sqlite3_uint64 value_bits) {
    double value = 0.0;
    memcpy(&value, &value_bits, sizeof(value));
    return sqlite3_bind_double(stmt, index, value);
}

sqlite3_uint64 gecko_sqlite3_column_double_bits(sqlite3_stmt *stmt, int iCol) {
    double value = sqlite3_column_double(stmt, iCol);
    sqlite3_uint64 bits = 0;
    memcpy(&bits, &value, sizeof(bits));
    return bits;
}
