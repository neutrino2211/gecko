#include <stddef.h>
#include <stdint.h>
#include <stdio.h>

struct List
{
  int* data;
  size_t el_size;
  uint64_t count;
};

uint64_t List_count(struct List* list) {
  return list->count;
}

uint64_t List$compute_count(struct List list) {
  return sizeof(list.data) / list.el_size;
}

void main() {

  int data[4] = {1,2,3,4};
  struct List list = {&data, sizeof(int),3};

  printf("Count: %d, Computed: %d\n", List_count(&list), List$compute_count(list));
}
