#include <stdio.h>
#include <stdlib.h>

char* get_greeting(char* name) {
  char* buf = malloc(128 * sizeof(char));
  sprintf(buf, "Hello, %s", name);

  printf("Address of string from C is 0x%x\n", buf);
  return buf;
}