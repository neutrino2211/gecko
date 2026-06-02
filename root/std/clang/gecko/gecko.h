#ifndef GECKO_STANDARD_H
#define GECKO_STANDARD_H

#include <stdint.h>

#define string char*

struct gecko_slice_t
{
  void* data;
  uint64_t type_size;
  uint64_t len;
};


#endif // GECKO_STANDARD_H
