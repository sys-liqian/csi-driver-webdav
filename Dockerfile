# Copyright 2023.

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM centos:centos7.9.2009

ARG binary=./bin/webdavplugin
COPY ${binary} /webdavplugin

RUN yum install epel-release -y && yum update -y && yum install davfs2 -y

ENTRYPOINT ["/webdavplugin"]