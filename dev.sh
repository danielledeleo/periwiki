make
if [ $? -eq 0 ]; then
    ./periwiki
else
    echo Build failed. Aborting.
fi