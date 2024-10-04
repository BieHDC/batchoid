package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

type ArrBoardDTO []BoardDTO

type BoardDTO struct {
	Board_id         string `json:"board_id"`
	Board_name       string `json:"board_name"`
	Created_at       string `json:"created_at"`
	Updated_at       string `json:"updated_at"`
	Deleted_at       string `json:"deleted_at"`
	Cover_image_name string `json:"cover_image_name"`
	Archived         bool   `json:"archived"`
	Is_private       bool   `json:"is_private"`
	Image_count      int    `json:"image_count"`
}

func (b BoardDTO) String() string {
	return fmt.Sprintf("ID: %s\n\tName: %s\n\tCover: %s\n\tCount: %d\n", b.Board_id, b.Board_name, b.Cover_image_name, b.Image_count)
}

func parseBoardDTO(r io.Reader) (BoardDTO, error) {
	d, err := io.ReadAll(r)
	if err != nil {
		return BoardDTO{}, fmt.Errorf("failed to unmarshal BoardDTO: %w", err)
	}

	var res BoardDTO
	err = json.Unmarshal(d, &res)
	if err != nil {
		return BoardDTO{}, fmt.Errorf("failed to unmarshal BoardDTO: %w", err)
	}

	return res, nil
}

func (g *gui) makeBoard(s string) (BoardDTO, error) {
	qq := fmt.Sprintf("/api/v1/boards/?board_name=%s&is_private=false", url.PathEscape(s))

	resp, err := g.servercall("POST", qq, nil, nil)
	if err != nil {
		return BoardDTO{}, err
	}
	defer resp.Body.Close()

	boarddto, err := parseBoardDTO(resp.Body)
	if err != nil {
		return BoardDTO{}, err
	}

	return boarddto, nil
}

func (g *gui) getBoards() (ArrBoardDTO, error) {
	resp, err := g.servercall("GET", "/api/v1/boards/?all=true", nil, nil)
	if err != nil {
		return ArrBoardDTO{}, err
	}
	defer resp.Body.Close()

	boardsresp, err := parseArrBoardDTO(resp.Body)
	if err != nil {
		return ArrBoardDTO{}, err
	}

	return boardsresp, nil
}

func parseArrBoardDTO(r io.Reader) (ArrBoardDTO, error) {
	d, err := io.ReadAll(r)
	if err != nil {
		return ArrBoardDTO{}, fmt.Errorf("failed to unmarshal PaginatedResults: %w", err)
	}

	var res ArrBoardDTO
	err = json.Unmarshal(d, &res)
	if err != nil {
		return ArrBoardDTO{}, fmt.Errorf("failed to unmarshal PaginatedResults: %w", err)
	}

	return res, nil
}

func (g *gui) getBoardID(vi *videoinfo) error {
	if vi.boardid == "" {
		// we need to query boards and search the id by name
		// get all boards
		boards, err := g.getBoards()
		if err != nil {
			return fmt.Errorf("failed to get boards: %w", err)
		}

		vi.boardid = "not found"
		for _, board := range boards {
			if vi.boardname == board.Board_name {
				vi.boardid = board.Board_id
				break
			}
		}
	}
	return nil
}
