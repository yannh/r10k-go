#!/bin/bash

set +e


export MODULEROOT=test-fixtures/modules
export SOURCEROOT=test-fixtures/source


function clean {
  rm -rf $MODULEROOT $SOURCEROOT
}

function create_testmodule {
    pushd $PWD
    mkdir -p $MODULEROOT/testmodule
    cd $MODULEROOT/testmodule

    echo "TestModule v1" > Readme
    git init
    git add Readme
    git commit -m "v1"
    git tag -a v1 -m "v1"

    echo "TestModule v2" > Readme
    git add Readme
    git commit -m "v2"
    git tag -a v2 -m "v2"

    echo "TestModule v3" > Readme
    git add Readme
    git commit -m "v3"
    git tag -a v3 -m "v3"

    popd
}

function create_environments {
    pushd $PWD
    mkdir -p $SOURCEROOT
    cd $SOURCEROOT


    echo "mod \"testmodule\", :git=>\"$MODULEROOT/testmodule\", :ref=>\"v1\"" > Puppetfile
    git init
    git add Puppetfile
    git commit -m "installv1"
    git tag -a installv1 -m "installv1"

    git checkout -b "validpuppetfile"
    echo "mod \"testmodule\", :git=>\"$MODULEROOT/testmodule\", :ref=>\"v3\"" > Puppetfile
    git add Puppetfile
    git commit -m "validpuppetfile"
    git checkout master

    git checkout -b "invalidpuppetfile"
    cat <<EOF  >Puppetfile
# mod, <module name>, <version or tag>, <source>
forge "http://forge.puppetlabs.com"

mod "voxpopuli/nginx"
  :git => "https://github.com/voxpupuli/puppet-nginx.git"
EOF
    git add Puppetfile
    git commit -m "invalidpuppetfile"
    git checkout master

    git checkout -b "installpath"
    echo "mod \"testmodule\", :git=>\"$MODULEROOT/testmodule\", :install_path=>\"test_install_path\"" > Puppetfile
    git add Puppetfile
    git commit -m "installpath"
    git checkout master


    popd
}

clean
create_testmodule >/dev/null 2>&1
create_environments > /dev/null 2>&1
