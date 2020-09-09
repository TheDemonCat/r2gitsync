package main

import (
	cli "github.com/jawher/mow.cli"
	"github.com/khorevaa/r2gitsync/internal/args"
	"github.com/khorevaa/r2gitsync/internal/opts"
)

// Sample use: vault creds reddit.com
func (app *Application) cmdSync(cmd *cli.Cmd) {

	cmd.LongDesc = `Выполнение синхронизации Хранилища 1С с git репозиторием`

	repo := SyncRepository{}

	opts.StringOpt(cmd, "storage-author u", "Администратор", "пользователь хранилища 1C конфигурации").
		Env("R2GITSYNC_STORAGE_USER GITSYNC_STORAGE_USER").
		Ptr(&repo.Repository.User)

	opts.StringOpt(cmd, "storage-pwd p", "", "пользователь хранилища 1C конфигурации").
		Env("R2GITSYNC_STORAGE_PASSWORD GITSYNC_STORAGE_PWD GITSYNC_STORAGE_PASSWORD").
		Ptr(&repo.Repository.Password)

	opts.StringOpt(cmd, "extension e ext", "", "имя расширения для работы с хранилищем расширения").
		Env("R2GITSYNC_EXTENSION GITSYNC_EXTENSION").
		Ptr(&repo.Extention)

	args.StringArg(cmd, "PATH", "", "Путь к хранилищу конфигурации 1С.").
		Env("R2GITSYNC_STORAGE_PATH GITSYNC_STORAGE_PATH").
		Ptr(&repo.Repository.Path)

	WorkdirArgPtr(&repo.WorkDir, cmd)

	cmd.Spec = "[OPTIONS] PATH [WORKDIR]"

	cmd.Action = func() {

		err := Sync(repo,
			WithInfobaseConfig(config.Infobase),
			WithTempDir(config.tempDir),
			WithV8Path(config.v8path),
			WithV8version(config.v8version),
			WithLicTryCount(5),
			WithPlugins(config.pluginsManager),
			//WithDomainEmail(config.),
		)

		failOnErr(err)

	}

	app.config.pluginsManager.RegistryOptions("sync", cmd)

}
