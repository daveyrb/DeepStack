package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	uuid "github.com/satori/go.uuid"

	"database/sql"
	"fmt"
	"path/filepath"

	"deepstack.io/server/middlewares"
	"deepstack.io/server/requests"
	"deepstack.io/server/response"
	"deepstack.io/server/structures"
	"deepstack.io/server/utils"
	_ "github.com/mattn/go-sqlite3"
)

var temp_path = "/deeptemp/"
var DATA_DIR = "/datastore"
var port = 5000

var db *sql.DB

var API_KEY = ""

var sub_key = ""

var state = true
var gpu = true

var expiring_date = time.Now()

var settings structures.Settings
var sub_data = structures.ActivationData{}
var config = structures.Config{PLATFORM: "DOCKER"}

var redis_client *redis.Client

func scene(c *gin.Context) {

	img_id := uuid.NewV4().String()
	req_id := uuid.NewV4().String()

	file, _ := c.FormFile("image")

	c.SaveUploadedFile(file, filepath.Join(temp_path, img_id))

	req_data := requests.RecognitionRequest{Imgid: img_id, Reqid: req_id, Reqtype: "scene"}
	req_string, _ := json.Marshal(req_data)

	redis_client.RPush("scene_queue", req_string)

	for true {

		output, _ := redis_client.Get(req_id).Result()

		if output != "" {

			var res response.RecognitionResponse
			json.Unmarshal([]byte(output), &res)

			if res.Success == false {

				var error_response response.ErrorResponseInternal

				json.Unmarshal([]byte(output), &error_response)

				final_res := response.ErrorResponse{Success: false, Error: error_response.Error}

				c.JSON(error_response.Code, final_res)
				return

			} else {
				c.JSON(200, res)
				return

			}

			break
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func detection(c *gin.Context, queue_name string) {

	nms := c.PostForm("min_confidence")

	if nms == "" {

		nms = "0.40"

	}

	img_id := uuid.NewV4().String()

	req_id := uuid.NewV4().String()

	detec_req := requests.DetectionRequest{Imgid: img_id, Minconfidence: nms, Reqtype: "detection", Reqid: req_id}

	face_req_string, _ := json.Marshal(detec_req)

	file, _ := c.FormFile("image")

	c.SaveUploadedFile(file, filepath.Join(temp_path, img_id))

	redis_client.RPush(queue_name, face_req_string)

	for true {

		output, _ := redis_client.Get(req_id).Result()

		if output != "" {

			var res response.DetectionResponse

			json.Unmarshal([]byte(output), &res)

			if res.Success == false {

				var error_response response.ErrorResponseInternal
				json.Unmarshal([]byte(output), &error_response)

				final_res := response.ErrorResponse{Success: false, Error: error_response.Error}

				c.JSON(error_response.Code, final_res)
				return

			} else {
				c.JSON(200, res)

				return
			}

			break
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func facedetection(c *gin.Context) {

	file, _ := c.FormFile("image")

	nms := c.PostForm("min_confidence")

	if nms == "" {

		nms = "0.55"

	}

	img_id := uuid.NewV4().String()
	req_id := uuid.NewV4().String()

	face_req := requests.FaceDetectionRequest{Imgid: img_id, Reqtype: "detect", Reqid: req_id, Minconfidence: nms}

	face_req_string, _ := json.Marshal(face_req)

	c.SaveUploadedFile(file, filepath.Join(temp_path, img_id))

	redis_client.RPush("face_queue", face_req_string)

	for true {

		output, _ := redis_client.Get(req_id).Result()

		if output != "" {

			var res response.FaceDetectionResponse
			json.Unmarshal([]byte(output), &res)

			if res.Success == false {

				var error_response response.ErrorResponseInternal
				json.Unmarshal([]byte(output), &error_response)

				final_res := response.ErrorResponse{Success: false, Error: error_response.Error}

				c.JSON(error_response.Code, final_res)

			} else {

				c.JSON(200, res)
				return
			}

			break
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func facerecognition(c *gin.Context) {

	file, _ := c.FormFile("image")

	threshold := c.PostForm("min_confidence")

	if threshold == "" {

		threshold = "0.67"

	}

	img_id := uuid.NewV4().String()
	req_id := uuid.NewV4().String()

	c.SaveUploadedFile(file, filepath.Join(temp_path, img_id))

	face_req := requests.FaceRecognitionRequest{Imgid: img_id, Reqtype: "recognize", Reqid: req_id, Minconfidence: threshold}

	face_req_string, _ := json.Marshal(face_req)

	redis_client.RPush("face_queue", face_req_string)

	for true {

		output, _ := redis_client.Get(req_id).Result()

		if output != "" {

			var res response.FaceRecognitionResponse
			json.Unmarshal([]byte(output), &res)

			if res.Success == false {

				var error_response response.ErrorResponseInternal
				json.Unmarshal([]byte(output), &error_response)

				final_res := response.ErrorResponse{Success: false, Error: error_response.Error}
				c.JSON(error_response.Code, final_res)
				return

			} else {

				c.JSON(200, res)
				return
			}

			break
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func faceregister(c *gin.Context) {

	userid := c.PostForm("userid")

	form, _ := c.MultipartForm()

	user_images := []string{}

	if form != nil {
		for filename, _ := range form.File {
			file, _ := c.FormFile(filename)
			img_id := uuid.NewV4().String()
			c.SaveUploadedFile(file, filepath.Join(temp_path, img_id))

			user_images = append(user_images, img_id)
		}
	}

	req_id := uuid.NewV4().String()

	request_body := requests.FaceRegisterRequest{Userid: userid, Images: user_images, Reqid: req_id, Reqtype: "register"}

	request_string, _ := json.Marshal(request_body)

	redis_client.RPush("face_queue", request_string)

	for true {

		output, _ := redis_client.Get(req_id).Result()

		if output != "" {

			var res response.FaceRegisterResponse
			json.Unmarshal([]byte(output), &res)

			if res.Success == false {

				var error_response response.ErrorResponseInternal
				json.Unmarshal([]byte(output), &error_response)

				final_res := response.ErrorResponse{Success: false, Error: error_response.Error}
				c.JSON(error_response.Code, final_res)
				return

			} else {
				c.JSON(200, res)
				return
			}

			break
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func facematch(c *gin.Context) {

	form, _ := c.MultipartForm()

	user_images := []string{}

	if form != nil {
		for filename, _ := range form.File {
			file, _ := c.FormFile(filename)
			img_id := uuid.NewV4().String()
			c.SaveUploadedFile(file, filepath.Join(temp_path, img_id))

			user_images = append(user_images, img_id)
		}
	}

	req_id := uuid.NewV4().String()

	request_body := requests.FaceMatchRequest{Images: user_images, Reqid: req_id, Reqtype: "match"}

	request_string, _ := json.Marshal(request_body)

	redis_client.RPush("face_queue", request_string)

	for true {

		output, _ := redis_client.Get(req_id).Result()

		if output != "" {

			var res response.FaceMatchResponse
			json.Unmarshal([]byte(output), &res)

			if res.Success == false {

				var error_response response.ErrorResponseInternal
				json.Unmarshal([]byte(output), &error_response)

				final_res := response.ErrorResponse{Success: false, Error: error_response.Error}
				c.JSON(error_response.Code, final_res)
				return

			} else {
				c.JSON(200, res)
				return
			}

			break
		}

		time.Sleep(1 * time.Millisecond)
	}

}

func listface(c *gin.Context) {

	TB_EMBEDDINGS := "TB_EMBEDDINGS"
	face2 := os.Getenv("VISION-FACE2")

	if face2 == "True" {

		TB_EMBEDDINGS = "TB_EMBEDDINGS2"

	}

	rows, _ := db.Query(fmt.Sprintf("select userid from %s", TB_EMBEDDINGS))

	var userids = []string{}
	for rows.Next() {

		var userid string
		rows.Scan(&userid)

		userids = append(userids, userid)

	}

	res := response.FacelistResponse{Success: true, Faces: userids}

	c.JSON(200, res)
	return

}

func deleteface(c *gin.Context) {

	userid := c.PostForm("userid")

	TB_EMBEDDINGS := "TB_EMBEDDINGS"
	face2 := os.Getenv("VISION-FACE2")

	if face2 == "True" {

		TB_EMBEDDINGS = "TB_EMBEDDINGS2"

	}

	trans, _ := db.Begin()

	stmt, _ := trans.Prepare(fmt.Sprintf("DELETE FROM %s WHERE userid=?", TB_EMBEDDINGS))

	defer stmt.Close()

	stmt.Exec(userid)

	trans.Commit()

	res := response.FaceDeleteResponse{Success: true}

	c.JSON(200, res)
	return

}

func register_model(c *gin.Context) {

	model_file, _ := c.FormFile("model")

	config_file, _ := c.FormFile("config")

	model_name := c.PostForm("name")

	MODEL_DIR := DATA_DIR + "/models/vision/" + model_name + "/"

	model_exists, _ := utils.PathExists(MODEL_DIR)
	message := "model updated"
	if model_exists == false {

		os.MkdirAll(MODEL_DIR, os.ModePerm)
		message = "model registered"

	}

	c.SaveUploadedFile(model_file, MODEL_DIR+"model.pb")
	c.SaveUploadedFile(config_file, MODEL_DIR+"config.json")
	res := response.ModelRegisterResponse{Success: true, Message: message}

	c.JSON(200, res)

}

func delete_model(c *gin.Context) {

	model_name := c.PostForm("name")

	MODEL_DIR := DATA_DIR + "/models/vision/" + model_name + "/"

	os.RemoveAll(MODEL_DIR)

	res := response.ModelDeleteResponse{Success: true, Message: "Model removed"}

	c.JSON(200, res)
	return

}

func list_models(c *gin.Context) {

	model_list, err := filepath.Glob(DATA_DIR + "/models/vision/*")

	models := []structures.ModelInfo{}

	if err == nil {

		for _, file := range model_list {

			model_name := filepath.Base(file)
			fileStat, _ := os.Stat(file + "/model.pb")
			size := float32(fileStat.Size()) / (1000 * 1000)

			model_info := structures.ModelInfo{Name: model_name, Dateupdated: fileStat.ModTime(), Modelsize: size}

			models = append(models, model_info)

		}

	}

	res := structures.AllModels{Models: models, Success: true}

	c.JSON(200, res)

	return

}

func single_request_loop(c *gin.Context, queue_name string) {

	img_id := uuid.NewV4().String()
	req_id := uuid.NewV4().String()

	file, _ := c.FormFile("image")

	c.SaveUploadedFile(file, filepath.Join(temp_path, img_id))

	req_data := requests.RecognitionRequest{Imgid: img_id, Reqid: req_id, Reqtype: "custom"}
	req_string, _ := json.Marshal(req_data)

	redis_client.RPush(queue_name, req_string)

	for true {

		output, _ := redis_client.Get(req_id).Result()

		if output != "" {

			var res response.RecognitionResponse
			json.Unmarshal([]byte(output), &res)

			if res.Success == false {

				var error_response response.ErrorResponseInternal

				json.Unmarshal([]byte(output), &error_response)

				final_res := response.ErrorResponse{Success: false, Error: error_response.Error}

				c.JSON(error_response.Code, final_res)
				return

			} else {
				c.JSON(200, res)
				return

			}

			break
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func backup(c *gin.Context) {

	file_id := uuid.NewV4().String() + ".zip"
	backup_name := "Backup_" + time.Now().Format("2006-01-02T15:04:05") + ".backup"

	output_file, _ := os.Create(temp_path + "/" + file_id)

	zip_archive := zip.NewWriter(output_file)

	models, err := filepath.Glob(DATA_DIR + "/models/vision/*")

	if err == nil {

		for _, file := range models {

			model_name := filepath.Base(file)

			utils.AddFileToZip(zip_archive, path.Join(file, "model.pb"), "models/vision/"+model_name+"/model.pb")
			utils.AddFileToZip(zip_archive, path.Join(file, "config.json"), "models/vision/"+model_name+"/config.json")

		}

	}

	utils.AddFileToZip(zip_archive, DATA_DIR+"/faceembedding.db", "faceembedding.db")

	zip_archive.Close()
	output_file.Close()

	data_file, _ := os.Open(temp_path + "/" + file_id)

	info, err := os.Stat(temp_path + "/" + file_id)

	if err != nil {

		fmt.Println(err)
	}

	contentLength := info.Size()

	contentType := "application/octet-stream"

	extraHeaders := map[string]string{
		"Content-Disposition": "attachment; filename=" + backup_name,
	}

	c.DataFromReader(200, contentLength, contentType, data_file, extraHeaders)

}

func restore(c *gin.Context) {

	backup_file, _ := c.FormFile("file")

	backup_path := temp_path + "/deepstack.backup"
	c.SaveUploadedFile(backup_file, backup_path)
	defer os.Remove(backup_path)

	zip_reader, err := zip.OpenReader(backup_path)

	if err != nil {

		response := response.ErrorResponse{Success: false, Error: "Invalid backup file"}

		c.JSON(200, response)

		return
	}

	defer zip_reader.Close()

	for _, f := range zip_reader.File {

		f_path := f.Name
		data, err := f.Open()
		if err != nil {

			fmt.Println(err)
		}

		fpath := path.Join(DATA_DIR, f_path)

		os.MkdirAll(filepath.Dir(fpath), os.ModePerm)

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())

		_, err = io.Copy(outFile, data)
		outFile.Close()

	}

	res := response.RestoreResponse{Success: true}

	c.JSON(200, res)

	return

}

func printfromprocess(cmd *exec.Cmd) {

	for true {

		out, err := cmd.StdoutPipe()
		if err == nil {

			outData, _ := ioutil.ReadAll(out)
			fmt.Println(string(outData))
			time.Sleep(1 * time.Second)

		}

	}

}

func printlogs() {

	face1 := os.Getenv("VISION-FACE")
	face2 := os.Getenv("VISION-FACE2")
	detection := os.Getenv("VISION-DETECTION")
	scene := os.Getenv("VISION-SCENE")

	if face1 == "True" || face2 == "True" {

		fmt.Println("/v1/vision/face")
		fmt.Println("---------------------------------------")
		fmt.Println("/v1/vision/face/recognize")
		fmt.Println("---------------------------------------")
		fmt.Println("/v1/vision/face/register")
		fmt.Println("---------------------------------------")
		fmt.Println("/v1/vision/face/match")
		fmt.Println("---------------------------------------")
		fmt.Println("/v1/vision/face/list")
		fmt.Println("---------------------------------------")
		fmt.Println("/v1/vision/face/delete")
		fmt.Println("---------------------------------------")

	}

	if detection == "True" {

		fmt.Println("/v1/vision/detection")
		fmt.Println("---------------------------------------")

	}

	if scene == "True" {

		fmt.Println("/v1/vision/scene")
		fmt.Println("---------------------------------------")

	}

	models, err := filepath.Glob(DATA_DIR + "/models/vision/*")

	custom := os.Getenv("VISION-CUSTOM")

	if config.PLATFORM == "RPI" {
		custom = os.Getenv("VISION_CUSTOM")
	}

	if err == nil && custom == "True" {

		for _, file := range models {
			model_name := filepath.Base(file)
			fmt.Println("v1/vision/custom/" + model_name)
			fmt.Println("---------------------------------------")
		}

	}

	fmt.Println("---------------------------------------")
	fmt.Println("v1/backup")
	fmt.Println("---------------------------------------")
	fmt.Println("v1/restore")

}

func home(c *gin.Context) {

	c.HTML(200, "index.html", gin.H{})

}

func initActivationRPI() {

	face1 := os.Getenv("VISION_FACE")
	face2 := os.Getenv("VISION_FACE2")
	detection := os.Getenv("VISION_DETECTION")
	scene := os.Getenv("VISION_SCENE")

	os.Setenv("VISION-FACE", face1)
	os.Setenv("VISION-FACE2", face2)
	os.Setenv("VISION-DETECTION", detection)
	os.Setenv("VISION-SCENE", scene)

}

func launchservices() {

}

func main() {

	APPDIR := os.Getenv("APPDIR")
	DATA_DIR = os.Getenv("DATA_DIR")
	if DATA_DIR == "" {
		DATA_DIR = "/datastore"
	}

	temp_path = os.Getenv("TEMP_PATH")
	if temp_path == "" {
		temp_path = "/deeptemp/"
	}

	os.Mkdir(filepath.Join(APPDIR, "logs"), 0755)
	os.Mkdir(DATA_DIR, 0755)
	os.Mkdir(temp_path, 0755)

	stdout, _ := os.Create(filepath.Join(APPDIR, "logs/stdout.txt"))

	defer stdout.Close()

	stderr, _ := os.Create(filepath.Join(APPDIR, "logs/stderr.txt"))

	defer stderr.Close()

	ctx := context.TODO()

	initScript := filepath.Join(APPDIR, "init.py")
	detectionScript := filepath.Join(APPDIR, "intelligencelayer/shared/detection.py")
	faceScript := filepath.Join(APPDIR, "intelligencelayer/shared/face.py")
	sceneScript := filepath.Join(APPDIR, "intelligencelayer/shared/scene.py")

	initcmd := exec.CommandContext(ctx, "bash", "-c", "python3 "+initScript)
	initcmd.Dir = APPDIR
	initcmd.Stdout = stdout
	initcmd.Stderr = stderr

	rediscmd := exec.CommandContext(ctx, "bash", "-c", "redis-server --daemonize yes")

	rediscmd.Stdout = stdout
	rediscmd.Stderr = stderr

	rediscmd.Run()
	initcmd.Run()

	if os.Getenv("VISION-DETECTION") == "True" {
		detectioncmd := exec.CommandContext(ctx, "bash", "-c", "python3 "+detectionScript)
		detectioncmd.Dir = filepath.Join(APPDIR, "intelligencelayer/shared")
		detectioncmd.Stdout = stdout
		detectioncmd.Stderr = stderr
		detectioncmd.Start()

	}

	if os.Getenv("VISION-FACE") == "True" {
		facecmd := exec.CommandContext(ctx, "bash", "-c", "python3 "+faceScript)
		facecmd.Dir = filepath.Join(APPDIR, "intelligencelayer/shared")
		facecmd.Stdout = stdout
		facecmd.Stderr = stderr
		facecmd.Start()

	}
	if os.Getenv("VISION-SCENE") == "True" {
		scenecmd := exec.CommandContext(ctx, "bash", "-c", "python3 "+sceneScript)
		scenecmd.Dir = filepath.Join(APPDIR, "intelligencelayer/shared")
		scenecmd.Stdout = stdout
		scenecmd.Stderr = stderr
		scenecmd.Start()

	}

	redis_client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	db, _ = sql.Open("sqlite3", filepath.Join(DATA_DIR, "faceembedding.db"))

	gin.SetMode(gin.ReleaseMode)

	server := gin.New()

	go utils.LogToServer(&sub_data)

	if config.PLATFORM == "RPI" {

		initActivationRPI()

	}

	admin_key := os.Getenv("ADMIN-KEY")
	api_key := os.Getenv("API-KEY")

	if config.PLATFORM == "RPI" {

		admin_key = os.Getenv("ADMIN_KEY")
		api_key = os.Getenv("API_KEY")

	}

	if admin_key != "" || api_key != "" {

		if admin_key != "" {

			settings.ADMIN_KEY = admin_key

		} else {

			settings.ADMIN_KEY = ""

		}

		if api_key != "" {

			settings.API_KEY = api_key

		} else {

			settings.API_KEY = ""

		}

	}

	server.Use(gin.Recovery())

	v1 := server.Group("/v1")
	v1.Use(gin.Logger())

	vision := v1.Group("/vision")
	vision.Use(middlewares.CheckApiKey(&sub_data, &settings))
	{
		vision.POST("/scene", middlewares.CheckScene(), middlewares.CheckImage(), scene)
		vision.POST("/detection", middlewares.CheckDetection(), middlewares.CheckImage(), middlewares.CheckConfidence(), func(c *gin.Context) {

			detection(c, "detection_queue")

		})

		facegroup := vision.Group("/face")
		facegroup.Use(middlewares.CheckFace())
		{
			facegroup.POST("/", middlewares.CheckImage(), middlewares.CheckConfidence(), facedetection)
			facegroup.POST("/recognize", middlewares.CheckImage(), middlewares.CheckConfidence(), facerecognition)
			facegroup.POST("/register", middlewares.CheckMultiImage(), faceregister)
			facegroup.POST("/match", middlewares.CheckFaceMatch(), facematch)
			facegroup.POST("/delete", middlewares.CheckUserID(), deleteface)
			facegroup.POST("/list", listface)

		}

		vision.POST("/addmodel", middlewares.CheckAdminKey(&sub_data, &settings), middlewares.CheckRegisterModel(&sub_data, DATA_DIR), register_model)
		vision.POST("/deletemodel", middlewares.CheckAdminKey(&sub_data, &settings), middlewares.CheckDeleteModel(DATA_DIR), delete_model)
		vision.POST("/listmodels", middlewares.CheckAdminKey(&sub_data, &settings), list_models)

		custom := vision.Group("/custom")
		custom.Use(middlewares.CheckImage())
		{

			models, err := filepath.Glob("/modelstore/detection/*.pt")

			if err == nil {

				for _, file := range models {

					model_name := filepath.Base(file)

					model_name = model_name[:strings.LastIndex(model_name, ".")]

					modelcmd := exec.CommandContext(ctx, "bash", "-c", "python3 "+detectionScript+" --model "+file+" --name "+model_name)
					modelcmd.Dir = filepath.Join(APPDIR, "intelligencelayer/shared")
					modelcmd.Stdout = stdout
					modelcmd.Stderr = stderr
					modelcmd.Start()

					custom.POST(model_name, func(c *gin.Context) {

						detection(c, model_name+"_queue")

					})

					fmt.Println("---------------------------------------")
					fmt.Println("v1/vision/custom/" + model_name)

				}

			}
		}

	}

	v1.POST("/backup", middlewares.CheckAdminKey(&sub_data, &settings), backup)
	v1.POST("/restore", middlewares.CheckAdminKey(&sub_data, &settings), middlewares.CheckRestore(), restore)

	server.Static("/assets", "./assets")
	server.LoadHTMLGlob("templates/*")
	server.GET("/", home)
	server.GET("/admin", home)

	port2 := strconv.Itoa(port)

	printlogs()
	server.Run(":" + port2)

}
