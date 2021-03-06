# Generated by go2rpm 1.2
%bcond_without check

# https://github.com/cosandr/go-check-updates
%global goipath         github.com/cosandr/go-check-updates
Version:                0
%global tag             v1.0-rc2

%gometa

%global common_description %{expand:
Check for updates and expose them through API or websocket.}

%global golicenses      LICENSE
%global godocs          README.md

Name:           go-check-updates
Release:        1%{?dist}
Summary:        Check for updates and expose them through API or websocket

License:        MIT
URL:            %{gourl}
Source0:        %{gosource}

# BuildRequires:  golang(github.com/alexflint/go-arg)
# BuildRequires:  golang(github.com/coreos/go-systemd/v22/activation)
# BuildRequires:  golang(github.com/gorilla/websocket)
# BuildRequires:  golang(github.com/sirupsen/logrus)
# BuildRequires:  golang(github.com/sirupsen/logrus/hooks/writer)
# BuildRequires:  golang(golang.org/x/sys/unix)

%description
%{common_description}

%gopkg

%prep
%goprep

%build
go mod vendor
%gobuild -o %{gobuilddir}/bin/%{name} %{goipath}
./setup.sh install systemd -n --pkg-name %{name} --tmp-path ./tmp --keep-tmp --no-log --no-cache
./setup.sh install env -n --pkg-name %{name} --tmp-path ./tmp --keep-tmp --no-log --no-cache

%install
%gopkginstall
install -m 0755 -vd                        %{buildroot}%{_bindir}
install -m 0755 -vp %{gobuilddir}/bin/*    %{buildroot}%{_bindir}/
install -m 0755 -vd                        %{buildroot}/etc/sysconfig
install -m 0640 -vp ./tmp/%{name}.env      %{buildroot}/etc/sysconfig/%{name}
install -m 0755 -vd                        %{buildroot}/usr/lib/systemd/system
install -m 0644 -vp ./tmp/%{name}.service  %{buildroot}/usr/lib/systemd/system/
install -m 0644 -vp ./tmp/%{name}.socket   %{buildroot}/usr/lib/systemd/system/

%if %{with check}
%check
%gocheck
%endif

%files
%license LICENSE
%doc README.md
%{_bindir}/*
%config(noreplace) /etc/sysconfig/%{name}
/usr/lib/systemd/system/%{name}.service
/usr/lib/systemd/system/%{name}.socket

%gopkgfiles

%changelog
* Mon Dec 07 2020 Andrei Costescu <andrei@costescu.no> - v1.0-rc2
- Discord notifications
- Use /etc/sysconfig for systemd env variables
* Wed Nov 04 2020 Andrei Costescu <andrei@costescu.no> - v1.0-rc1
- Initial package