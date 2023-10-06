/*
 *
 *  * Copyright 2022 CloudWeGo Authors
 *  *
 *  * Licensed under the Apache License, Version 2.0 (the "License");
 *  * you may not use this file except in compliance with the License.
 *  * You may obtain a copy of the License at
 *  *
 *  *     http://www.apache.org/licenses/LICENSE-2.0
 *  *
 *  * Unless required by applicable law or agreed to in writing, software
 *  * distributed under the License is distributed on an "AS IS" BASIS,
 *  * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  * See the License for the specific language governing permissions and
 *  * limitations under the License.
 *
 */

package service

import (
	"context"
	"fmt"
	"github.com/cloudwego/cwgo/platform/server/cmd/agent/internal/svc"
	"github.com/cloudwego/cwgo/platform/server/shared/consts"
	"github.com/cloudwego/cwgo/platform/server/shared/kitex_gen/agent"
	"github.com/cloudwego/cwgo/platform/server/shared/logger"
	"github.com/cloudwego/cwgo/platform/server/shared/utils"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
)

const (
	successMsgGenerateCode = "" // TODO: to be filled...
)

type GenerateCodeService struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
} // NewGenerateCodeService new GenerateCodeService
func NewGenerateCodeService(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateCodeService {
	return &GenerateCodeService{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Run create note info
func (s *GenerateCodeService) Run(req *agent.GenerateCodeReq) (resp *agent.GenerateCodeRes, err error) {
	// get idl info by idl id
	idl, err := s.svcCtx.DaoManager.Idl.GetIDL(req.IdlId)
	if err != nil {
		logger.Logger.Error("get idl info failed", zap.Error(err))
		return &agent.GenerateCodeRes{
			Code: http.StatusInternalServerError,
			Msg:  "internal err",
		}, nil
	}

	// get repository info by repository id
	repo, err := s.svcCtx.DaoManager.Repository.GetRepository(idl.RepositoryId)
	if err != nil {
		logger.Logger.Error("get repo info failed", zap.Error(err))
		return &agent.GenerateCodeRes{
			Code: http.StatusInternalServerError,
			Msg:  "internal err",
		}, nil
	}

	// get repo client
	client, err := s.svcCtx.RepoManager.GetClient(repo.Id)
	if err != nil {
		logger.Logger.Error("get repo client failed", zap.Error(err), zap.Int64("repo_id", repo.Id))
		return &agent.GenerateCodeRes{
			Code: http.StatusInternalServerError,
			Msg:  "internal err",
		}, nil
	}

	idlPid, owner, repoName, err := client.ParseUrl(idl.MainIdlPath)
	if err != nil {
		logger.Logger.Error("parse repo url failed", zap.Error(err))
		return &agent.GenerateCodeRes{
			Code: http.StatusInternalServerError,
			Msg:  "internal err",
		}, nil
	}

	// Create temp dir
	tempDir, err := ioutil.TempDir("", strconv.FormatInt(repo.Id, 64))
	if err != nil {
		logger.Logger.Error("create temp dir failed", zap.Error(err))
		return &agent.GenerateCodeRes{
			Code: http.StatusInternalServerError,
			Msg:  "internal err",
		}, nil
	}
	defer os.RemoveAll(tempDir)

	idlFile, err := client.GetFile(owner, repoName, idlPid, consts.MainRef)
	if err != nil {
		logger.Logger.Error("get idl files failed", zap.Error(err))
		return &agent.GenerateCodeRes{
			Code: http.StatusInternalServerError,
			Msg:  "internal err",
		}, nil
	}

	// Create a thrift file in a temporary folder
	filePathOnDisk := fmt.Sprintf("%s/%s", tempDir, idlFile.Name)
	if err := ioutil.WriteFile(filePathOnDisk, idlFile.Content, 0644); err != nil {
		logger.Logger.Error("write file failed", zap.Error(err))
		return &agent.GenerateCodeRes{
			Code: http.StatusInternalServerError,
			Msg:  "internal err",
		}, nil
	}

	err = s.svcCtx.Generator.Generate(idlFile.Name, idl.ServiceName, tempDir)
	if err != nil {
		logger.Logger.Error("generate file failed", zap.Error(err))
		return &agent.GenerateCodeRes{
			Code: http.StatusInternalServerError,
			Msg:  "internal err",
		}, nil
	}

	fileContentMap := make(map[string][]byte)
	if err := utils.ProcessFolders(fileContentMap, tempDir, "kitex_gen", "rpc"); err != nil {
		return &agent.GenerateCodeRes{
			Code: http.StatusInternalServerError,
			Msg:  "internal err",
		}, nil
	}

	err = client.PushFilesToRepository(fileContentMap, owner, idl.MainIdlPath, consts.MainRef, "generated by cwgo")
	if err != nil {
		return &agent.GenerateCodeRes{
			Code: http.StatusInternalServerError,
			Msg:  "internal err",
		}, nil
	}

	resp.Code = 0
	resp.Msg = successMsgGenerateCode

	return resp, nil
}
