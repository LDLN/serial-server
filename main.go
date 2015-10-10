/*
 *  Copyright 2014-2015 LDLN
 *
 *  This file is part of LDLN Serial Server.
 *
 *  LDLN Serial Server is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  any later version.
 *
 *  LDLN Serial Server is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with LDLN Serial Server.  If not, see <http://www.gnu.org/licenses/>.
 */
package main

import (
	"log"
	"github.com/tarm/serial"
	"labix.org/v2/mgo"
	"encoding/json"
	"github.com/ldln/core/cryptoWrapper"
)

func main() {

	// connect to mongodb
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer session.Close()
	
	// connect to port
	c := &serial.Config{Name: "/dev/ttyS0", Baud: 38400}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}
	
	// write message	
	n, err := s.Write([]byte("LDLN Serial is listening"))
	if err != nil {
		log.Fatal(err)
	}

	// read messages
	for {
        buf := make([]byte, 2048)
        n, err = s.Read(buf)
        if err != nil {
                log.Fatal(err)
        }
        log.Printf("Incoming on serial: %q", buf[:n])
		
		// split by double pipes /
		// first part is user:pass
		// second part is object type
		// third part is key/value pairs, separated by pipes |
		// each key/value pair is seperated by :
		// i.e. user:pass/memo/timestamp:xxx|gps:xxx|humidity:xxx|temp:xxx|soilsensor:xxx|dfo10sensor:xxx|gyro:xxx
		// i.e. user:pass/object_type/timestamp:xxx|gps:xxx|humidity:xxx|temp:xxx|soilsensor:xxx|dfo10sensor:xxx|gyro:xxx
		
		// convert string to JSON to map[string]interface{}
		v := make(map[string]interface{})
		err := json.Unmarshal(buf[:n], &v)
		if err != nil {
			log.Printf("Not a JSON object")
        } else {
			
			// get auth
			username := v["username"].(string)
			password := v["password"].(string)
			
			// create object
			object_map := make(map[string]interface{})
			object_map["uuid"] = v["uuid"].(string)
			object_map["object_type"] = v["object_type"].(string)
			object_map["time_modified_since_creation"] = v["time_modified_since_creation"].(float64)
			
			// encrypted payload
			// object_map["key_value_pairs"] = v["key_value_pairs"].(string)
			
			// plaintext to be encrypted payload
			byt, err := json.Marshal(v["key_value_pairs_plaintext"].(map[string]interface{}))
			if err != nil {
				panic(err)
			}
			log.Printf(string(byt[:]))
			
			dek := cryptoWrapper.GetKeyFromUsernamePassword(username, password)
			ciphertext := cryptoWrapper.Encrypt(dek, byt)
			if ciphertext != nil {
				object_map["key_value_pairs"] = ciphertext
			
				// db insert
				mc := session.DB("landline").C("SyncableObjects")
				err = mc.Insert(object_map)
				if err != nil {
					panic(err)
				}
				log.Printf("Inserted object %q into database.", v["uuid"].(string))
			} else {
				s.Write([]byte("Encryption failed"))
				log.Printf("Encryption failed")
			}
		}
	}
}