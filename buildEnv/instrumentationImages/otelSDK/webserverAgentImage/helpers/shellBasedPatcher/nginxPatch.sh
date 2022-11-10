#! /bin/bash
set -x

conf=${1}
load_module_directive=${2}
include_config_directive=${3}

cp ${conf} ${conf}.bck

sed -i '' '1s,^,'${load_module_directive}'\n,g' ${conf}

if ! grep -q 'conf.d/*.conf' "${conf}"; then
  http_line_no=$(grep -n -E '[[:space:]]*http[[:space:]]*{[[:space:]]*' ${conf} | sed 's/:.*//')
  http_line_no=$(expr $http_line_no + 1)
  sed -i '' ${http_line_no}'s,^,'${include_config_directive}'\n,g' ${conf}
fi

